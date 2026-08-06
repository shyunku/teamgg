import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { compactRows } from "../compact.js";

export interface KillEvent { t: number; victimId: string; killerId: string | null; }
export interface ObjectiveEvent { t: number; type: string; lane: string | null; tier: string | null; defendingTeamId: number | null; killerId: string | null; }
export interface TimedEvent { t: number; [key: string]: unknown; }
export interface GoldSnapshot { t: number; team100: number; team200: number; diff: number; [key: string]: unknown; }
export interface GoldSwing { before: GoldSnapshot; after: GoldSnapshot; delta: number; leadReversed: boolean; }
export interface EventCandidate { id: string; kind: string; start: number; end: number; score: number; baseScore: number; kills: KillEvent[]; objectives: ObjectiveEvent[]; goldSwing: GoldSwing | null; }

export function objectiveImpact(type: string, tier: string | null): number {
  if (type === "baron-killed") return 500;
  if (type === "dragon-killed") return 300;
  if (type === "rift-herald-killed") return 225;
  if (type === "inhibitor-destroyed") return 300;
  if (type === "turret-destroyed") return tier === "inner" ? 225 : tier === "nexus" ? 300 : 175;
  return 150;
}
function nearestAtOrBefore(rows: GoldSnapshot[], t: number): GoldSnapshot | undefined {
  let result: GoldSnapshot | undefined;
  for (const row of rows) { if (row.t > t) break; result = row; }
  return result;
}

function nearestAtOrAfter(rows: GoldSnapshot[], t: number): GoldSnapshot | undefined { return rows.find((row) => row.t >= t); }

export function calculateGoldSwing(rows: GoldSnapshot[], start: number, end: number): GoldSwing | null {
  const before = nearestAtOrBefore(rows, Math.max(0, start - 10));
  const after = nearestAtOrAfter(rows, end + 15) ?? nearestAtOrBefore(rows, end + 30);
  if (!before || !after || after.t <= before.t) return null;
  const delta = after.diff - before.diff;
  return { before, after, delta, leadReversed: before.diff !== 0 && after.diff !== 0 && Math.sign(before.diff) !== Math.sign(after.diff) };
}

export function buildCandidates(kills: KillEvent[], objectives: ObjectiveEvent[], goldTimeline: GoldSnapshot[] = []): EventCandidate[] {
  const candidates: EventCandidate[] = [];
  for (const kill of [...kills].sort((a, b) => a.t - b.t)) {
    const current = candidates.at(-1);
    if (current?.kind === "fight" && kill.t - current.end <= 20) { current.end = kill.t; current.kills.push(kill); current.baseScore += 100; }
    else candidates.push({ id: "", kind: "fight", start: kill.t, end: kill.t, score: 100, baseScore: 100, kills: [kill], objectives: [], goldSwing: null });
  }
  for (const objective of [...objectives].sort((a, b) => a.t - b.t)) {
    const weight = objectiveImpact(objective.type, objective.tier);
    const preceding = [...candidates].reverse().find((candidate) => objective.t >= candidate.start - 10 && objective.t <= candidate.end + 90);
    if (preceding) { preceding.objectives.push(objective); preceding.end = Math.max(preceding.end, objective.t); preceding.baseScore += weight; preceding.kind = "fight-to-objective"; }
    else candidates.push({ id: "", kind: objective.type, start: objective.t, end: objective.t, score: weight, baseScore: weight, kills: [], objectives: [objective], goldSwing: null });
  }
  for (const candidate of candidates) {
    candidate.goldSwing = calculateGoldSwing(goldTimeline, candidate.start, candidate.end);
    const swing = candidate.goldSwing;
    const goldImpact = swing ? Math.min(400, Math.round(Math.abs(swing.delta) / 10)) + (swing.leadReversed ? 250 : 0) : 0;
    candidate.score = candidate.baseScore + goldImpact;
  }
  return [...candidates].sort((a, b) => b.score - a.score).slice(0, 5).sort((a, b) => a.start - b.start);
}

export async function writeEventDossiers(root: string, kills: KillEvent[], spellCasts: TimedEvent[], damageEvents: TimedEvent[], movement: TimedEvent[], items: TimedEvent[], objectives: ObjectiveEvent[], wards: TimedEvent[], health: TimedEvent[], visibility: TimedEvent[] = [], goldTimeline: GoldSnapshot[] = []) {
  const destination = join(root, "events"); await mkdir(destination, { recursive: true });
  const candidates = buildCandidates(kills, objectives, goldTimeline); const summaries: Record<string, unknown>[] = [];
  await Promise.all(candidates.map(async (event, index) => {
    event.id = `e${index + 1}`; const from = Math.max(0, event.start - 30), to = event.end + 20; const inside = (row: TimedEvent) => row.t >= from && row.t <= to;
    const dossier = {
      schema: "teamgg-replay-event-dossier-v3", id: event.id, kind: event.kind, start: event.start, end: event.end,
      impact: { score: event.score, baseScore: event.baseScore, goldSwing: event.goldSwing }, evidenceWindow: { from, to },
      kills: compactRows(["t", "victimId", "killerId"], event.kills), objectives: compactRows(["t", "type", "lane", "tier", "defendingTeamId", "killerId"], event.objectives),
      spells: compactRows(["t", "casterId", "slot", "targetId", "sourceX", "sourceZ", "targetX", "targetZ"], spellCasts.filter(inside) as Array<Record<string, unknown>>),
      damage: compactRows(["t", "sourceId", "targetId", "damage", "damageType"], damageEvents.filter(inside) as Array<Record<string, unknown>>),
      movement: compactRows(["t", "playerId", "speed", "x", "z"], movement.filter(inside) as Array<Record<string, unknown>>),
      items: compactRows(["t", "playerId", "action", "itemId", "slot", "count"], items.filter(inside) as Array<Record<string, unknown>>),
      wards: compactRows(["t", "ownerId", "wardType", "slot", "x", "z"], wards.filter(inside) as Array<Record<string, unknown>>),
      health: compactRows(["t", "playerId", "health", "maxHealth", "healthPct", "change"], health.filter(inside) as Array<Record<string, unknown>>),
      visibility: compactRows(["t", "playerId", "state", "channel"], visibility.filter(inside) as Array<Record<string, unknown>>),
      gold: compactRows(["t", "team100", "team200", "diff"], goldTimeline.filter(inside)),
    };
    summaries.push({ id: event.id, kind: event.kind, start: event.start, end: event.end, kills: event.kills.length, objectives: event.objectives.length, score: event.score, goldReversal: event.goldSwing?.leadReversed ?? null, evidenceFrom: from, evidenceTo: to });
    await writeFile(join(destination, `${event.id}.json`), `${JSON.stringify(dossier, null, 2)}\n`, "utf8");
  }));
  summaries.sort((a, b) => String(a.id).localeCompare(String(b.id)));
  await writeFile(join(destination, "index.json"), `${JSON.stringify(compactRows(["id", "kind", "start", "end", "kills", "objectives", "score", "goldReversal", "evidenceFrom", "evidenceTo"], summaries), null, 2)}\n`, "utf8");
}
