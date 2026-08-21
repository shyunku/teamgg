import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { DecodeDetails } from "rofl-parser";
import type { ReplayPlayer } from "../domain.js";
import { compactPlayers, compactRows } from "../compact.js";
import type { RoflInspection } from "./metadata.js";
import { writeEventDossiers } from "./event-dossiers.js";
import { writeAiPreparation } from "./ai-preparation.js";

type RecordValue = Record<string, unknown>;

export interface RefinedEvent {
  t: number;
  playerId: string;
  level?: number;
}
export interface RefinedReplay {
  players: ReplayPlayer[];
  deathEvents: RefinedEvent[];
  kills: Array<{ t: number; victimId: string; killerId: string | null }>;
  levelUps: RefinedEvent[];
  packetFiles: number;
  packetRows: number;
  packetFilesSkipped: number;
  artifactRoot: string;
}

function value(record: RecordValue, ...keys: string[]): unknown {
  for (const key of keys) if (record[key] !== undefined && record[key] !== null) return record[key];
  return undefined;
}

function text(record: RecordValue, ...keys: string[]): string | null {
  const raw = value(record, ...keys);
  if (raw === undefined) return null;
  const result = String(raw).trim();
  return result === "" ? null : result.slice(0, 100);
}

function numberValue(record: RecordValue, ...keys: string[]): number | null {
  const result = Number(value(record, ...keys));
  return Number.isFinite(result) ? result : null;
}

function boolValue(record: RecordValue, ...keys: string[]): boolean | null {
  const raw = value(record, ...keys);
  if (typeof raw === "boolean") return raw;
  const normalized = String(raw ?? "").toLowerCase();
  if (["win", "true", "1", "yes"].includes(normalized)) return true;
  if (["fail", "loss", "false", "0", "no"].includes(normalized)) return false;
  return null;
}

function buildPlayer(stats: RecordValue, index: number, paramKey: number | null): ReplayPlayer {
  const gameName = text(stats, "RIOT_ID_GAME_NAME", "GAME_NAME", "NAME") ?? `Player ${index + 1}`;
  const tagLine = text(stats, "RIOT_ID_TAG_LINE", "TAG_LINE");
  return {
    participantIndex: index,
    playerId: `p${index + 1}`,
    riotId: tagLine ? `${gameName}#${tagLine}` : gameName,
    champion: text(stats, "SKIN", "CHAMPION_NAME"),
    championId: numberValue(stats, "SKIN_ID", "CHAMPION_ID"),
    team: numberValue(stats, "TEAM", "TEAM_ID"),
    won: boolValue(stats, "WIN"), position: text(stats, "TEAM_POSITION", "INDIVIDUAL_POSITION", "PLAYER_ROLE"),
    level: numberValue(stats, "LEVEL", "CHAMP_LEVEL"), kills: numberValue(stats, "CHAMPIONS_KILLED", "KILLS"),
    deaths: numberValue(stats, "NUM_DEATHS", "DEATHS"), assists: numberValue(stats, "ASSISTS"),
    goldEarned: numberValue(stats, "GOLD_EARNED"), goldSpent: numberValue(stats, "GOLD_SPENT"),
    visionScore: numberValue(stats, "VISION_SCORE"), totalDamageToChampions: numberValue(stats, "TOTAL_DAMAGE_DEALT_TO_CHAMPIONS"),
    totalDamageTaken: numberValue(stats, "TOTAL_DAMAGE_TAKEN"), minionsKilled: numberValue(stats, "MINIONS_KILLED"),
    neutralMinionsKilled: numberValue(stats, "NEUTRAL_MINIONS_KILLED"), wardsPlaced: numberValue(stats, "WARD_PLACED", "WARDS_PLACED"),
    wardsKilled: numberValue(stats, "WARD_KILLED", "WARDS_KILLED"), objectivesStolen: numberValue(stats, "OBJECTIVES_STOLEN"),
    turretKills: numberValue(stats, "TURRETS_KILLED", "TURRET_KILLS"),
    items: Array.from({ length: 7 }, (_, item) => numberValue(stats, `ITEM${item}`) ?? 0).filter((item) => item > 0),
    entityId: null,
    ...(paramKey === null ? {} : { paramKey: `0x${paramKey.toString(16)}` }),
    metrics: {
      championDamage: numberValue(stats, "TOTAL_DAMAGE_DEALT_TO_CHAMPIONS"), damageTaken: numberValue(stats, "TOTAL_DAMAGE_TAKEN"),
      damageToObjectives: numberValue(stats, "TOTAL_DAMAGE_DEALT_TO_OBJECTIVES"), damageToTurrets: numberValue(stats, "DAMAGE_DEALT_TO_TURRETS"),
      timeDeadSeconds: numberValue(stats, "TOTAL_TIME_SPENT_DEAD"), ccTimeSeconds: numberValue(stats, "TOTAL_TIME_CC_DEALT"),
      healing: numberValue(stats, "TOTAL_HEAL"), shieldingAllies: numberValue(stats, "TOTAL_DAMAGE_SHIELDED_ON_TEAMMATES"),
      controlWardsBought: numberValue(stats, "CONTROL_WARDS_BOUGHT_IN_GAME"), objectiveStealAssists: numberValue(stats, "OBJECTIVE_STEAL_ASSISTS"),
      afk: boolValue(stats, "WAS_AFK"), leaver: boolValue(stats, "LEAVER"),
    },
  };
}

function paramKey(raw: unknown): number | null {
  const number = typeof raw === "string" && raw.startsWith("0x") ? Number.parseInt(raw, 16) : Number(raw);
  return Number.isSafeInteger(number) ? number & 0xffff : null;
}

async function parserPlayerParamKeys(decodedPath: string, count: number): Promise<(number | null)[]> {
  try {
    const parsed = JSON.parse(await readFile(join(decodedPath, "metadata.json"), "utf8")) as { metadata?: { players?: unknown[] } };
    const players = Array.isArray(parsed.metadata?.players) ? parsed.metadata.players : [];
    return Array.from({ length: count }, (_, index) => {
      const player = players[index];
      return player && typeof player === "object" ? paramKey((player as RecordValue).paramHint) : null;
    });
  } catch { return Array.from({ length: count }, () => null); }
}

async function packetRows(path: string): Promise<RecordValue[]> {
  try {
    const document = JSON.parse(await readFile(path, "utf8")) as { data?: unknown };
    return Array.isArray(document.data) ? document.data.filter((row): row is RecordValue => typeof row === "object" && row !== null) : [];
  } catch { return []; }
}

function compactValue(value: unknown, key = "", playerByKey?: Map<number, string>): unknown {
  if (key === "unparsedTail") return undefined;
  if (typeof value === "string" && /(netid|entityid|param)$/i.test(key)) { const mapped = playerByKey?.get(paramKey(value) ?? -1); return mapped ?? value; }
  if (typeof value === "number") return Number.isInteger(value) ? value : Math.round(value * 1000) / 1000;
  if (Array.isArray(value)) return value.slice(0, 32).map((entry) => compactValue(entry, "", playerByKey));
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value as RecordValue).flatMap(([childKey, child]) => { const compact = compactValue(child, childKey, playerByKey); return compact === undefined ? [] : [[childKey, compact]]; }));
  return value;
}
async function autoRefinePackets(packetDir: string, destination: string, playerByKey: Map<number, string>) {
  await mkdir(destination, { recursive: true }); const entries: Array<Record<string, unknown>> = [];
  for (const file of await readdir(packetDir)) { if (!file.endsWith(".json")) continue; try { const document = JSON.parse(await readFile(join(packetDir, file), "utf8")) as RecordValue; const level = Number(document.level ?? 0); const data = Array.isArray(document.data) ? document.data.filter((row): row is RecordValue => typeof row === "object" && row !== null) : []; if (level <= 0 || !data.some((row) => row.namedParameters && typeof row.namedParameters === "object")) continue; const events = data.flatMap((row) => { const t = eventTime(row); const params = compactValue(row.namedParameters, "", playerByKey); if (t === null || !params || typeof params !== "object") return []; const actorId = playerByKey.get(paramKey(row.param) ?? -1) ?? null; return [{ t, actorId, params }]; }); if (!events.length) continue; const output = file.replace(/\.json$/, ".refined.json"); await writeFile(join(destination, output), `${JSON.stringify(compactRows(["t", "actorId", "params"], events), null, 2)}\n`, "utf8"); entries.push({ file: `parsed/${output}`, sourcePacket: document.packetName ?? file, sourceParseLevel: level, events: events.length, semanticValidation: "automatically retained: namedParameters present; unparsedTail removed; known player NetIds normalized" }); } catch { /* unreadable packet is omitted */ } }
  return entries;
}
function eventTime(row: RecordValue): number | null {
  const time = Number(row.timestamp);
  return Number.isFinite(time) ? Math.round(time * 1000) / 1000 : null;
}

export function extractHeroDeaths(
  rows: RecordValue[],
  playerByKey: ReadonlyMap<number, string>,
): Array<{ t: number; victimId: string; killerId: string | null }> {
  const mapped = (raw: unknown): string | undefined => {
    const key = paramKey(raw);
    return key === null ? undefined : playerByKey.get(key);
  };

  return rows.flatMap((row) => {
    const t = eventTime(row);
    const named = row.namedParameters as RecordValue | null;
    // On patches with full semantic overlays, prefer the explicitly decoded
    // victim. On 16.16 the hero-death packet's block param is the victim NetID;
    // this is validated against every player's post-game death count below.
    const victimId = mapped(named?.victimEntityNetId) ?? mapped(row.param);
    const killerId = mapped(named?.killerNetId) ?? null;
    return t === null || !victimId ? [] : [{ t, victimId, killerId }];
  });
}

function sum(players: ReplayPlayer[], field: keyof ReplayPlayer): number {
  return players.reduce((total, player) => total + (typeof player[field] === "number" ? player[field] : 0), 0);
}

export async function refineReplay(inspection: RoflInspection, decoded: DecodeDetails): Promise<RefinedReplay> {
  const root = join(decoded.path, "refined");
  const packetRoot = join(root, "packets");
  await mkdir(packetRoot, { recursive: true });
  const keys = await parserPlayerParamKeys(decoded.path, inspection.stats.length);
  const players = inspection.stats.map((stats, index) => buildPlayer(stats, index, keys[index] ?? null));
  const playerByKey = new Map<number, string>();
  for (const player of players) if (player.paramKey) {
    const key = Number.parseInt(player.paramKey, 16);
    if (playerByKey.has(key)) playerByKey.delete(key); else playerByKey.set(key, player.playerId);
  }
  const packetDir = join(decoded.path, "packets");
  const [deathRows, levelRows, heroDeathRows, movementRows, spellRows, damageRows, buyRows, useItemRows, npcDieRows, wardRows, healthRows, neutralCreateRows, npcMapDeathRows, visibilityEnterRows, visibilityLeaveRows] = await Promise.all([
    packetRows(join(packetDir, "S2C_UpdateDeathTimer_s.json")),
    packetRows(join(packetDir, "NPC_LevelUp_s.json")),
    packetRows(join(packetDir, "NPC_Hero_Die_s.json")),
    packetRows(join(packetDir, "CUSTOM_Movement.json")),
    packetRows(join(packetDir, "NPC_CastSpellAns_s.json")),
    packetRows(join(packetDir, "UnitApplyDamage_s.json")),
    packetRows(join(packetDir, "BuyItemAns_s.json")),
    packetRows(join(packetDir, "UseItemAns_s.json")),
    packetRows(join(packetDir, "NPC_Die_Broadcast_s.json")),
    packetRows(join(packetDir, "CUSTOM_ObjectInitialState.json")),
    packetRows(join(packetDir, "CUSTOM_ReplicateFields.json")),
    packetRows(join(packetDir, "CUSTOM_CreateNeutral.json")),
    packetRows(join(packetDir, "NPC_Die_MapView_s.json")),
    packetRows(join(packetDir, "S2C_OnEnterTeamVisibility_s.json")),
    packetRows(join(packetDir, "S2C_OnLeaveTeamVisibility_s.json")),
  ]);
  const toPlayer = (row: RecordValue) => { const key = paramKey(row.param); return key === null ? undefined : playerByKey.get(key); };
  const kills = extractHeroDeaths(heroDeathRows, playerByKey);
  const deathEvents = (kills.length > 0 ? kills.map(({ t, victimId }) => ({ t, playerId: victimId })) : deathRows.flatMap((row) => { const t = eventTime(row); const playerId = toPlayer(row); return t === null || !playerId ? [] : [{ t, playerId }]; }))
    .filter((event, index, events) => index === 0 || `${event.t}:${event.playerId}` !== `${events[index - 1]!.t}:${events[index - 1]!.playerId}`);
  const seenLevels = new Set<string>();
  const levelUps = levelRows.flatMap((row) => {
    const t = eventTime(row); const playerId = toPlayer(row); const level = Array.isArray(row.parameters) ? Number(row.parameters[0]) : NaN;
    const key = `${playerId}:${level}`;
    if (t === null || !playerId || !Number.isInteger(level) || level < 2 || level > 20 || seenLevels.has(key)) return [];
    seenLevels.add(key); return [{ t, playerId, level }];
  });
  const mapped = (raw: unknown) => { const key = paramKey(raw); return key === null ? undefined : playerByKey.get(key); };
  const movementSamples: Array<{ t: number; playerId: string; speed: number; x: number; z: number }> = [];
  const sampledMovement = new Map<string, number>();
  for (const row of movementRows) { const t = eventTime(row); const entries = (row.namedParameters as RecordValue | null)?.movements; if (t === null || !Array.isArray(entries)) continue; for (const entry of entries) { if (!entry || typeof entry !== "object") continue; const movement = entry as RecordValue; const playerId = mapped(movement.entityId); const waypoints = movement.waypoints; const point = Array.isArray(waypoints) ? waypoints.at(-1) : undefined; if (!playerId || !Array.isArray(point) || !Number.isFinite(Number(point[0])) || !Number.isFinite(Number(point[1]))) continue; if (t - (sampledMovement.get(playerId) ?? -Infinity) < 10) continue; sampledMovement.set(playerId, t); movementSamples.push({ t, playerId, speed: Math.round(Number(movement.speed) || 0), x: Math.round(Number(point[0])), z: Math.round(Number(point[1])) }); } }
  const spellCasts = spellRows.flatMap((row) => { const t = eventTime(row); const named = row.namedParameters as RecordValue | null; const casterId = mapped(named?.casterNetId); const targetId = mapped(named?.targetNetId) ?? null; const source = named?.sourcePosition as RecordValue | null; const target = named?.targetPosition as RecordValue | null; if (t === null || !casterId || !named || !Number.isInteger(Number(named.spellSlot))) return []; return [{ t, casterId, slot: Number(named.spellSlot), targetId, sourceX: Math.round(Number(source?.x) || 0), sourceZ: Math.round(Number(source?.z) || 0), targetX: Math.round(Number(target?.x) || 0), targetZ: Math.round(Number(target?.z) || 0) }]; });
  const damageEvents: Array<{ t: number; sourceId: string; targetId: string; damage: number; damageType: string | null }> = [];
  const damageMap = new Map<string, { sourceId: string; targetId: string; damageType: string | null; totalDamage: number; hits: number }>();
  for (const row of damageRows) { const named = row.namedParameters as RecordValue | null; const sourceId = mapped(named?.sourceNetId); const targetId = mapped(named?.targetNetId); const amount = Number(named?.damage); if (!sourceId || !targetId || !Number.isFinite(amount) || amount <= 0) continue; const t = eventTime(row); const damageType = typeof named?.damageType === "string" ? named.damageType : null; if (t !== null) damageEvents.push({ t, sourceId, targetId, damage: Math.round(amount * 100) / 100, damageType }); const key = `${sourceId}:${targetId}:${damageType}`; const current = damageMap.get(key) ?? { sourceId, targetId, damageType, totalDamage: 0, hits: 0 }; current.totalDamage += amount; current.hits += 1; damageMap.set(key, current); }
  const wards = wardRows.flatMap((row) => { const t = eventTime(row); const n = row.namedParameters as RecordValue | null; const ownerId = mapped(n?.wardOwnerNetId); const position = n?.position as RecordValue | null; if (t === null || !n || !ownerId || !position) return []; return [{ t, ownerId, wardType: String(n.wardType ?? "unknown"), slot: Number(n.sourceItemSlot ?? -1), x: Math.round(Number(position.x) || 0), z: Math.round(Number(position.z) || 0) }]; });
  const healthSnapshots = healthRows.flatMap((row) => { const t = eventTime(row); const n = row.namedParameters as RecordValue | null; const updates = n?.healthUpdates; if (t === null || !Array.isArray(updates)) return []; return updates.flatMap((raw) => { const update = raw as RecordValue; const playerId = mapped(update.netId); const current = Number(update.currentHealth), max = Number(update.maxHealth); if (!playerId || !Number.isFinite(current) || !Number.isFinite(max) || max <= 0 || current < 0 || current > max * 1.1) return []; return [{ t, playerId, health: Math.round(current), maxHealth: Math.round(max), healthPct: Math.round(current / max * 1000) / 10, change: typeof update.healthChange === "string" ? update.healthChange : null }]; }); });  const neutralByEntity = new Map<string, RecordValue>();
  for (const row of neutralCreateRows) { const n = row.namedParameters as RecordValue | null; if (n && ["dragon", "baron", "rift-herald"].includes(String(n.objectiveType))) neutralByEntity.set(String(n.entityNetId), n); }
  const neutralObjectives = npcMapDeathRows.flatMap((row) => { const t = eventTime(row); const n = row.namedParameters as RecordValue | null; const created = n ? neutralByEntity.get(String(n.victimEntityNetId)) : undefined; if (t === null || !n || !created) return []; return [{ t, type: `${String(created.objectiveType)}-killed`, lane: null, tier: typeof created.dragonType === "string" ? created.dragonType : null, defendingTeamId: null, killerId: mapped(n.killerNetId) ?? null }]; });  const objectives = npcDieRows.flatMap((row) => { const t = eventTime(row); const named = row.namedParameters as RecordValue | null; if (t === null || !named || (named.event !== "turret-destroyed" && named.event !== "inhibitor-destroyed")) return []; return [{ t, type: String(named.event), lane: typeof named.lane === "string" ? named.lane : null, tier: typeof named.tier === "string" ? named.tier : null, defendingTeamId: Number(named.defendingTeamId) || null, killerId: mapped(named.directKillerNetId) ?? null }]; });
  const visibilitySeen = new Set<string>();
  const visibility = [...visibilityEnterRows, ...visibilityLeaveRows].flatMap((row) => {
    const t = eventTime(row); const n = row.namedParameters as RecordValue | null; const playerId = mapped(n?.entityNetId);
    const state = n?.event === "visibility-entered" ? "visible" : n?.event === "visibility-left" ? "hidden" : null;
    const channel = Number(n?.visibilityChannelCode); const key = `${t}:${playerId}:${state}:${channel}`;
    if (t === null || !playerId || !state || visibilitySeen.has(key)) return []; visibilitySeen.add(key);
    return [{ t, playerId, state, channel: Number.isFinite(channel) ? channel : null }];
  }).sort((a, b) => a.t - b.t);
  const goldTimeline: Array<{ t: number; team100: number; team200: number; diff: number }> = [];  const itemEvents = [...buyRows, ...useItemRows].flatMap((row) => { const t = eventTime(row); const playerId = toPlayer(row); const named = row.namedParameters as RecordValue | null; if (t === null || !playerId || !named) return []; return [{ t, playerId, action: String(named.action ?? "unknown"), itemId: Number.isFinite(Number(named.itemId)) ? Number(named.itemId) : null, slot: Number.isFinite(Number(named.slot)) ? Number(named.slot) : null, count: Number(named.itemsInSlot ?? 0) }]; });  const deathCount = new Map<string, number>();
  for (const event of deathEvents) deathCount.set(event.playerId, (deathCount.get(event.playerId) ?? 0) + 1);
  const deathValidation = players.every((player) => (player.deaths ?? 0) === (deathCount.get(player.playerId) ?? 0));
  const gameLengthMs = numberValue(inspection.metadata, "gameLength");
  const metadata = {
    schema: "teamgg-rofl-refined-v1", source: { fileName: inspection.fileName, sha256: inspection.sha256, fileSize: inspection.fileSize, gameVersion: inspection.gameVersion, patch: inspection.patch, gameId: text(inspection.metadata, "gameId", "gameID"), durationSeconds: gameLengthMs === null ? null : gameLengthMs / 1000 },
    idScheme: { playerId: "p1..p10 are replay-local identifiers", paramHint: "lower 16 bits are matched; upper 0x100 changes map to the same player" },
    players,
    teams: compactRows(["team", "won", "kills", "deaths", "assists", "goldEarned", "visionScore", "championDamage"], [...new Set(players.map((p) => p.team).filter((team): team is number => team !== null))].sort().map((team) => { const members = players.filter((p) => p.team === team); return { team, won: members[0]?.won ?? null, kills: sum(members, "kills"), deaths: sum(members, "deaths"), assists: sum(members, "assists"), goldEarned: sum(members, "goldEarned"), visionScore: sum(members, "visionScore"), championDamage: sum(members, "totalDamageToChampions") }; })),
    decoder: { compact: true, artifactReused: decoded.artifactReused, packetRows: Number(decoded.counts.packetDataCount ?? 0), packetFiles: Number(decoded.counts.packetFileCount ?? 0) },
  };
  const autoEntries = await autoRefinePackets(packetDir, join(packetRoot, "parsed"), playerByKey);
  const allObjectives = [...objectives, ...neutralObjectives].sort((a, b) => a.t - b.t);
  await writeEventDossiers(root, kills, spellCasts, damageEvents, movementSamples, itemEvents, allObjectives, wards, healthSnapshots, visibility, goldTimeline);
  await writeFile(join(packetRoot, "wards.json"), `${JSON.stringify(compactRows(["t", "ownerId", "wardType", "slot", "x", "z"], wards), null, 2)}\n`, "utf8");
  await writeFile(join(packetRoot, "health.json"), `${JSON.stringify(compactRows(["t", "playerId", "health", "maxHealth", "healthPct", "change"], healthSnapshots), null, 2)}\n`, "utf8");
  await writeFile(join(packetRoot, "objectives.json"), `${JSON.stringify(compactRows(["t", "type", "lane", "tier", "defendingTeamId", "killerId"], allObjectives), null, 2)}\n`, "utf8");
  await writeFile(join(packetRoot, "visibility.json"), `${JSON.stringify(compactRows(["t", "playerId", "state", "channel"], visibility), null, 2)}\n`, "utf8");
  await writeFile(join(packetRoot, "gold-timeline.json"), `${JSON.stringify({ availability: "unavailable", reason: "decoded packets have no verified time-series gold/experience semantics", timeline: compactRows(["t", "team100", "team200", "diff"], goldTimeline) }, null, 2)}\n`, "utf8");
  await writeFile(join(root, "metadata.json"), `${JSON.stringify(metadata, null, 2)}\n`, "utf8");
  await writeAiPreparation(root);
  const manifest = {
    schema: "teamgg-rofl-refined-packets-v1",
    selected: [
      { file: "objectives.json", sourcePacket: "PKT_NPC_Die_Broadcast_s/PKT_CUSTOM_CreateNeutral/PKT_NPC_Die_MapView_s", sourceParseLevel: 1, events: allObjectives.length, semanticValidation: "decoded structures plus dragon, herald and baron deaths joined by entity NetId" },
      { file: "visibility.json", sourcePacket: "PKT_S2C_OnEnterTeamVisibility_s/PKT_S2C_OnLeaveTeamVisibility_s", sourceParseLevel: 1, events: visibility.length, semanticValidation: "mapped player visibility transitions; detector ward identity is not decoded" },
      { file: "gold-timeline.json", sourcePacket: null, sourceParseLevel: null, events: 0, semanticValidation: "explicitly unavailable; final match gold must not be presented as a timeline" },
      { file: "wards.json", sourcePacket: "PKT_CUSTOM_ObjectInitialState", sourceParseLevel: 1, events: wards.length, semanticValidation: "mapped owner and decoded ward placement position" },
      { file: "health.json", sourcePacket: "PKT_CUSTOM_ReplicateFields", sourceParseLevel: 1, events: healthSnapshots.length, semanticValidation: "mapped player current/max health with invalid values filtered" },
      { file: "movement-snapshots.json", sourcePacket: "PKT_CUSTOM_Movement", sourceParseLevel: 1, events: movementSamples.length, semanticValidation: "mapped player waypoint snapshots sampled at 10-second minimum intervals" },
      { file: "spell-casts.json", sourcePacket: "PKT_NPC_CastSpellAns_s", sourceParseLevel: 1, events: spellCasts.length, semanticValidation: "mapped caster plus decoded slot and coordinates" },
      { file: "damage-summary.json", sourcePacket: "PKT_UnitApplyDamage_s", sourceParseLevel: 2, events: damageMap.size, semanticValidation: "mapped player-to-player damage aggregated by source, target, and type" },
      { file: "items.json", sourcePacket: "PKT_BuyItemAns_s/PKT_UseItemAns_s", sourceParseLevel: 1, events: itemEvents.length, semanticValidation: "mapped player inventory actions" },
      { file: "kills.json", sourcePacket: "PKT_NPC_Hero_Die_s", sourceParseLevel: 1, events: kills.length, semanticValidation: deathValidation ? "victim NetIDs map to replay-local players and all death counts match final stats; killer remains null when no verified semantic field exists" : "partial; victim counts do not fully match final stats" },
      { file: "deaths.json", sourcePacket: "PKT_NPC_Hero_Die_s/PKT_S2C_UpdateDeathTimer_s", sourceParseLevel: 0, events: deathEvents.length, semanticValidation: deathValidation ? "all player death counts match final stats" : "partial; treat as low confidence" },
      { file: "level-ups.json", sourcePacket: "PKT_NPC_LevelUp_s", sourceParseLevel: 2, events: levelUps.length, semanticValidation: "player mapping and level range validated" },
    ],
    automaticallyRetained: autoEntries,
    excluded: [{ sourcePacket: "PKT_S2C_BuyItemAns", reason: "level-1/raw compact arrays are not semantically reliable yet" }, { sourcePacket: "all other packets", reason: "not yet validated for gameplay semantics; omitted rather than sending raw parser output to AI" }],
  };
  await Promise.all([
    writeFile(join(packetRoot, "movement-snapshots.json"), `${JSON.stringify(compactRows(["t", "playerId", "speed", "x", "z"], movementSamples), null, 2)}\n`, "utf8"),
    writeFile(join(packetRoot, "spell-casts.json"), `${JSON.stringify(compactRows(["t", "casterId", "slot", "targetId", "sourceX", "sourceZ", "targetX", "targetZ"], spellCasts), null, 2)}\n`, "utf8"),
    writeFile(join(packetRoot, "damage-summary.json"), `${JSON.stringify(compactRows(["sourceId", "targetId", "damageType", "totalDamage", "hits"], [...damageMap.values()].map((entry) => ({ ...entry, totalDamage: Math.round(entry.totalDamage * 100) / 100 }))), null, 2)}\n`, "utf8"),
    writeFile(join(packetRoot, "items.json"), `${JSON.stringify(compactRows(["t", "playerId", "action", "itemId", "slot", "count"], itemEvents), null, 2)}\n`, "utf8"),
    writeFile(join(packetRoot, "kills.json"), `${JSON.stringify(compactRows(["t", "victimId", "killerId"], kills), null, 2)}\n`, "utf8"),
    writeFile(join(packetRoot, "deaths.json"), `${JSON.stringify(compactRows(["t", "playerId"], deathEvents), null, 2)}\n`, "utf8"),
    writeFile(join(packetRoot, "level-ups.json"), `${JSON.stringify(compactRows(["t", "playerId", "level"], levelUps), null, 2)}\n`, "utf8"),
    writeFile(join(packetRoot, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`, "utf8"),
  ]);
  return { players, deathEvents, kills, levelUps, packetFiles: Number(decoded.counts.packetFileCount ?? 0), packetRows: Number(decoded.counts.packetDataCount ?? 0), packetFilesSkipped: Math.max(0, Number(decoded.counts.packetFileCount ?? 0) - 2), artifactRoot: root };
}
