import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { compactRows } from "../compact.js";

type Row = Record<string, unknown>;
function rows(table: unknown): Row[] {
  const value = table as { fields?: string[]; data?: unknown[][] };
  return Array.isArray(value?.fields) && Array.isArray(value?.data)
    ? value.data.map((row) => Object.fromEntries(value.fields!.map((field, index) => [field, row[index]])))
    : [];
}
function latestAtOrBefore(items: Row[], key: string, t: number): Row[] {
  const map = new Map<string, Row>();
  for (const item of items) if (Number(item.t) <= t) map.set(String(item[key]), item);
  return [...map.values()];
}
function aggregateSpells(spells: Row[]) {
  const values = new Map<string, number>();
  for (const spell of spells) { const key = `${spell.casterId}:${spell.slot}`; values.set(key, (values.get(key) ?? 0) + 1); }
  return [...values].map(([key, count]) => { const [playerId, slot] = key.split(":"); return { playerId, slot: Number(slot), count }; });
}
function aggregateDamage(damage: Row[]) {
  const values = new Map<string, { totalDamage: number; hits: number }>();
  for (const hit of damage) {
    const key = `${hit.sourceId}:${hit.targetId}:${hit.damageType}`;
    const current = values.get(key) ?? { totalDamage: 0, hits: 0 };
    current.totalDamage += Number(hit.damage ?? 0); current.hits += 1; values.set(key, current);
  }
  return [...values].map(([key, value]) => {
    const [sourceId, targetId, damageType] = key.split(":");
    return { sourceId, targetId, damageType, totalDamage: Math.round(value.totalDamage), hits: value.hits };
  });
}
function aggregateVisibility(visibility: Row[]) {
  const values = new Map<string, { entered: number; left: number; lastState: string; lastT: number }>();
  for (const row of visibility) {
    const key = String(row.playerId); const current = values.get(key) ?? { entered: 0, left: 0, lastState: "", lastT: 0 };
    if (row.state === "visible") current.entered += 1; else current.left += 1;
    current.lastState = String(row.state); current.lastT = Number(row.t); values.set(key, current);
  }
  return [...values].map(([playerId, value]) => ({ playerId, ...value }));
}
function impactLevel(score: number) { return score >= 900 ? "very-high" : score >= 600 ? "high" : score >= 350 ? "medium" : "low"; }
function distance(a: Row, b: Row) { return Math.hypot(Number(a.x) - Number(b.x), Number(a.z) - Number(b.z)); }
function incidentCenter(kills: Row[], objectives: Row[], movement: Row[], eventStart: number): Row | null {
  const focusPlayer = String(kills[0]?.victimId ?? objectives[0]?.killerId ?? "");
  if (!focusPlayer) return null;
  return latestAtOrBefore(movement.filter((row) => row.playerId === focusPlayer), "playerId", eventStart)[0] ?? null;
}
function presence(positions: Row[], center: Row | null, teams: Map<string, number>) {
  if (!center) return { radius: 2500, center: null, nearby: compactRows(["playerId", "team", "distance"], []) };
  const nearby = positions.map((row) => ({ playerId: String(row.playerId), team: teams.get(String(row.playerId)) ?? null, distance: Math.round(distance(row, center)) })).filter((row) => row.distance <= 2500);
  return { radius: 2500, center: { x: center.x, z: center.z }, nearby: compactRows(["playerId", "team", "distance"], nearby) };
}
function confidence(sources: string[], gaps: string[]) {
  const evidence = sources.length >= 7 ? "high" : sources.length >= 4 ? "medium" : "low";
  const interpretation = evidence === "low" ? "low" : gaps.includes("gold-timeline-unavailable") || gaps.includes("vision-detector-unavailable") ? "medium" : evidence;
  return { evidence, interpretation, sources, gaps };
}
export async function writeAiPreparation(refinedRoot: string) {
  const metadata = JSON.parse(await readFile(join(refinedRoot, "metadata.json"), "utf8"));
  const players: Row[] = Array.isArray(metadata.players) ? metadata.players : [];
  const teams = new Map(players.map((player) => [String(player.playerId), Number(player.team)]));
  const eventsRoot = join(refinedRoot, "events");
  const index = JSON.parse(await readFile(join(eventsRoot, "index.json"), "utf8"));
  const eventIds = rows(index).map((row) => String(row.id));
  const events: Row[] = [];
  const eventValidation: Row[] = [];

  for (const id of eventIds) {
    const raw = JSON.parse(await readFile(join(eventsRoot, `${id}.json`), "utf8"));
    const kills = rows(raw.kills), objectives = rows(raw.objectives), spells = rows(raw.spells), damage = rows(raw.damage);
    const movement = rows(raw.movement), health = rows(raw.health), wards = rows(raw.wards), items = rows(raw.items), visibility = rows(raw.visibility), gold = rows(raw.gold);
    const start = Number(raw.start), end = Number(raw.end);
    const positionsBefore = latestAtOrBefore(movement, "playerId", start);
    const positionsAfter = latestAtOrBefore(movement, "playerId", end);
    const healthBefore = latestAtOrBefore(health, "playerId", start);
    const healthAfter = latestAtOrBefore(health, "playerId", end);
    const center = incidentCenter(kills, objectives, movement, start);
    const sources = [
      kills.length ? "kills" : null, objectives.length ? "objectives" : null, damage.length ? "damage" : null,
      spells.length ? "spells" : null, positionsBefore.length >= 8 ? "movement" : null,
      healthBefore.length >= 8 ? "health" : null, wards.length ? "wards" : null, visibility.length ? "visibility" : null,
      gold.length ? "gold" : null,
    ].filter((source): source is string => Boolean(source));
    const gaps = [
      positionsBefore.length < 8 ? "insufficient-position-coverage" : null,
      healthBefore.length < 8 ? "insufficient-health-coverage" : null,
      wards.length === 0 ? "no-ward-placement-in-window" : null,
      "vision-detector-unavailable",
      gold.length === 0 ? "gold-timeline-unavailable" : null,
      "experience-timeline-unavailable",
    ].filter((gap): gap is string => Boolean(gap));
    const requiredChecks = {
      outcome: kills.length + objectives.length > 0,
      positions: positionsBefore.length >= 8 && positionsAfter.length >= 8,
      health: healthBefore.length >= 8 && healthAfter.length >= 8,
      combat: damage.length > 0 && spells.length > 0,
      vision: wards.length > 0 && visibility.length > 0,
    };
    const passedChecks = Object.values(requiredChecks).filter(Boolean).length;
    eventValidation.push({ id, passedChecks, totalChecks: Object.keys(requiredChecks).length, sufficient: passedChecks === Object.keys(requiredChecks).length, missing: Object.entries(requiredChecks).filter(([, passed]) => !passed).map(([name]) => name) });    const impact = raw.impact as Row | undefined;
    const score = Number(impact?.score ?? 0);
    events.push({
      id, kind: raw.kind, start, end,
      impact: { score, level: impactLevel(score), baseScore: impact?.baseScore ?? score, goldSwing: impact?.goldSwing ?? null },
      confidence: confidence(sources, gaps),
      outcomes: { kills: compactRows(["t", "victimId", "killerId"], kills), objectives: compactRows(["t", "type", "lane", "tier", "defendingTeamId", "killerId"], objectives) },
      combat: {
        spellUsage: compactRows(["playerId", "slot", "count"], aggregateSpells(spells)),
        damage: compactRows(["sourceId", "targetId", "damageType", "totalDamage", "hits"], aggregateDamage(damage)),
        healthBefore: compactRows(["playerId", "health", "maxHealth", "healthPct"], healthBefore),
        healthAfter: compactRows(["playerId", "health", "maxHealth", "healthPct"], healthAfter),
      },
      positioning: {
        before: compactRows(["playerId", "speed", "x", "z"], positionsBefore),
        after: compactRows(["playerId", "speed", "x", "z"], positionsAfter),
        teamPresence: presence(positionsBefore, center, teams),
      },
      vision: {
        wardPlacements: compactRows(["t", "ownerId", "wardType", "x", "z"], wards.slice(-16)),
        playerVisibility: compactRows(["playerId", "entered", "left", "lastState", "lastT"], aggregateVisibility(visibility)),
        limitation: "visibility channels are observed, but detectorWardNetId is unavailable in the decoded packet semantics",
      },
      inventoryActions: compactRows(["t", "playerId", "action", "itemId", "slot", "count"], items.slice(-20)),
    });
  }

  const output = {
    schema: "teamgg-ai-event-input-v3",
    purpose: "Evidence-first replay analysis. The model must distinguish observed facts from cautious interpretation.",
    players: compactRows(["playerId", "team", "champion", "position", "won"], players),
    economy: {
      availability: "unavailable",
      reason: "decoded namedParameters contain no verified time-series gold or experience values",
      teamGoldTimeline: compactRows(["t", "team100", "team200", "diff"], []),
      leadReversals: compactRows(["t", "beforeDiff", "afterDiff", "relatedEventId"], []),
      scoringRuleWhenAvailable: "event score adds up to 400 from absolute gold swing and 250 when the lead sign reverses",
    },
    limits: {
      maxEvents: 5, windowBeforeSeconds: 30, windowAfterSeconds: 20, presenceRadius: 2500,
      wardEventsPerEvent: 16, itemEventsPerEvent: 20,
      health: "latest state at event start/end per player", movement: "latest position at event start/end per player",
      damage: "source-target-type aggregate", spells: "caster-slot aggregate",
    },
    semantics: {
      facts: "Event evidence comes from verified namedParameters, plus hero-death packet params whose player death counts are cross-checked against post-game stats.",
      interpretation: "Intent, causality, visibility advantage and alternative actions are hypotheses, never facts.",
      confidence: "Impact and evidence confidence are independent.",
      desiredOutput: "chronological event -> cause -> consequence -> actionable alternative, with impact and confidence separated",
    },
    validation: {
      readyForAi: eventValidation.every((event) => event.sufficient === true),
      criteria: "outcome + >=8 player positions before/after + >=8 health states before/after + combat + vision evidence",
      events: compactRows(["id", "passedChecks", "totalChecks", "sufficient", "missing"], eventValidation),
      warnings: ["No time-series gold/experience; economy-based conclusions must not be asserted.", "Visibility detector identity is unavailable; ward-caused vision must not be asserted."],
    },
    events,
  };
  const root = join(refinedRoot, "..", "process", "prompt-assets"); await mkdir(root, { recursive: true });
  await writeFile(join(root, "06-event-dossiers.json"), `${JSON.stringify(output, null, 2)}\n`, "utf8");
}
