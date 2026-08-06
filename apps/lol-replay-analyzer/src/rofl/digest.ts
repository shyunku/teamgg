import { readFile } from "node:fs/promises";
import { join } from "node:path";
import type { ReplayDigest, TeamDigest } from "../domain.js";
import type { RoflInspection } from "./metadata.js";
import { refineReplay } from "./refine.js";
import type { DecodeDetails } from "rofl-parser";

function sum<T extends object>(items: T[], field: keyof T): number {
  return items.reduce((total, item) => total + (typeof item[field] === "number" ? item[field] as number : 0), 0);
}

function deathWindows(deaths: Array<{ t: number; playerId: string }>) {
  const windows: Array<{ start: number; end: number; playerIds: string[] }> = [];
  for (const death of deaths) {
    const current = windows.at(-1);
    if (current && death.t - current.end <= 20) { current.end = death.t; current.playerIds.push(death.playerId); }
    else windows.push({ start: death.t, end: death.t, playerIds: [death.playerId] });
  }
  return windows.filter((window) => window.playerIds.length >= 2);
}

export async function buildDigest(inspection: RoflInspection, decoded: DecodeDetails): Promise<ReplayDigest> {
  const refined = await refineReplay(inspection, decoded);
  const eventDossiers = JSON.parse(await readFile(join(decoded.path, "process", "prompt-assets", "06-event-dossiers.json"), "utf8"));
  const teamIds = [...new Set(refined.players.map((p) => p.team).filter((team): team is number => team !== null))].sort();
  const teams: TeamDigest[] = teamIds.map((team) => {
    const players = refined.players.filter((player) => player.team === team);
    return {
      team, won: players.find((player) => player.won !== null)?.won ?? null,
      kills: sum(players, "kills"), deaths: sum(players, "deaths"), assists: sum(players, "assists"),
      goldEarned: sum(players, "goldEarned"), visionScore: sum(players, "visionScore"),
      totalDamageToChampions: sum(players, "totalDamageToChampions"), minionsKilled: sum(players, "minionsKilled"),
      objectivesStolen: sum(players, "objectivesStolen"), turretKills: sum(players, "turretKills"),
    };
  });
  const gameLength = Number(inspection.metadata.gameLength);
  const deaths = refined.deathEvents;
  return {
    schema: "teamgg-replay-digest-v2",
    replay: {
      fileName: inspection.fileName, fileSize: inspection.fileSize, sha256: inspection.sha256,
      gameVersion: inspection.gameVersion, patch: inspection.patch,
      gameId: typeof inspection.metadata.gameId === "string" ? inspection.metadata.gameId : null,
      durationSeconds: Number.isFinite(gameLength) ? gameLength / 1000 : null,
      decodedPacketRows: refined.packetRows, packetFiles: refined.packetFiles,
    },
    players: refined.players,
    teams,
    timeline: {
      deaths,
      kills: refined.kills,
      levelMilestones: refined.levelUps.filter((event) => event.level === 6 || event.level === 11 || event.level === 16) as Array<{ t: number; playerId: string; level: number }>,
      deathWindows: deathWindows(deaths),
    },
    eventDossiers,
    dataQuality: {
      parserOutputFormat: "compact", decoderArtifactReused: decoded.artifactReused,
      packetEventsIncluded: refined.deathEvents.length + refined.kills.length + refined.levelUps.length,
      packetFilesSkipped: refined.packetFilesSkipped,
      warnings: [
        "종료 통계는 고신뢰 데이터이며, 타임라인은 paramHint 정규화 후 검증된 사망·레벨업 이벤트만 포함합니다.",
        "사건 입력은 검증된 킬·구조물·중립 오브젝트·위치·체력·피해·스킬·와드·시야·아이템 근거를 포함합니다.",
        "시간대별 골드·경험치와 특정 와드의 시야 제공자는 파서에서 확인되지 않아 명시적으로 제외합니다.",
      ],
    },
  };
}
