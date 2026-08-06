export type AnalysisStage =
  | "queued"
  | "upload-complete"
  | "replay-inspection"
  | "executable-resolution"
  | "decoder-preparation"
  | "replay-decoding"
  | "digest-building"
  | "prompt-building"
  | "ai-analysis"
  | "complete";

export interface ProgressUpdate {
  stage: AnalysisStage;
  message: string;
  progress?: number;
  detail?: Record<string, unknown>;
}

export interface ReplayPlayer {
  participantIndex: number;
  playerId: string;
  riotId: string;
  champion: string | null;
  championId: number | null;
  team: number | null;
  won: boolean | null;
  position: string | null;
  level: number | null;
  kills: number | null;
  deaths: number | null;
  assists: number | null;
  goldEarned: number | null;
  goldSpent: number | null;
  visionScore: number | null;
  totalDamageToChampions: number | null;
  totalDamageTaken: number | null;
  minionsKilled: number | null;
  neutralMinionsKilled: number | null;
  wardsPlaced: number | null;
  wardsKilled: number | null;
  objectivesStolen: number | null;
  turretKills: number | null;
  items: number[];
  entityId: string | null;
  paramKey?: string;
  metrics: Record<string, number | boolean | null>;
}

export interface TeamDigest {
  team: number;
  won: boolean | null;
  kills: number;
  deaths: number;
  assists: number;
  goldEarned: number;
  visionScore: number;
  totalDamageToChampions: number;
  minionsKilled: number;
  objectivesStolen: number;
  turretKills: number;
}

export interface PacketEventDigest {
  timestamp: number;
  packetName: string;
  actor: string | null;
  parameters: Record<string, unknown> | unknown[] | null;
}

export interface ReplayDigest {
  schema: "teamgg-replay-digest-v2";
  replay: {
    fileName: string;
    fileSize: number;
    sha256: string;
    gameVersion: string;
    patch: string;
    gameId: string | null;
    durationSeconds: number | null;
    decodedPacketRows: number;
    packetFiles: number;
  };
  players: ReplayPlayer[];
  teams: TeamDigest[];
  timeline: {
    deaths: Array<{ t: number; playerId: string }> ;
    kills: Array<{ t: number; victimId: string; killerId: string | null }> ;
    levelMilestones: Array<{ t: number; playerId: string; level: number }> ;
    deathWindows: Array<{ start: number; end: number; playerIds: string[] }> ;
  };
  eventDossiers: unknown;
  dataQuality: {
    parserOutputFormat: "compact";
    decoderArtifactReused: boolean;
    packetEventsIncluded: number;
    packetFilesSkipped: number;
    warnings: string[];
  };
}

export interface AnalysisOptions {
  language: string;
  focusPlayer?: string;
}

export interface AnalysisResult {
  requestId: string;
  digest: ReplayDigest;
  analysis: string;
  model: string;
  retainedArtifacts?: { root: string; refined: string; process: string };
}

export type ProgressReporter = (update: ProgressUpdate) => void;
export type TextDeltaReporter = (delta: string) => void;
