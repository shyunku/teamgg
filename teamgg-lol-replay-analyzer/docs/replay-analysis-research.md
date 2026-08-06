# Replay Analysis Research — KR-8326522247

## Current artifact

- Replay: `KR-8326522247.rofl`
- Game version: `16.15.801.3452`
- Duration: 2,227.9 seconds
- Parsed game-data packets: 1,646,505
- Snapshot packets: 437,536
- Packet files: 216
- Structural parameter decoding: 929,488 blocks succeeded; 218 failed

The retained output is under `.work/research-KR-8326522247/KR-8326522247/`.

| Artifact | Size | Use now |
|---|---:|---|
| `metadata.json` | 24.8 MB | Yes, but only the normalized `metadata.players` and `metadata.rawStats` sections. The file also embeds the original ROFL as Base64 and must never be forwarded whole. |
| `game-data.json` | 176.2 MB | No. It is the raw chronological packet stream and is too large. |
| `snapshots.json` | 34.5 MB | Not yet. Snapshot payloads are currently raw bytes rather than a meaningful game state. |
| `packets/*.json` | 216 files | Selectively. Only verified packet families should be summarized. |

## Verified identity mapping

`metadata.players` gives each participant a `paramHint` such as `0x400000AE` through `0x400000B7`.

Packet rows carry a `param` value. A player's replay entity can be recreated with different upper bits, e.g. the same Samira appeared as both `0x400000B6` and `0x400001B6`. For a row to be attributed to a player:

1. Convert `param` from hex to uint32.
2. Compare its lower 16 bits to the lower 16 bits of the ten known player hints.
3. Attribute only when that lower-16-bit value maps uniquely to one participant.
4. Keep the source packet family and confidence with the derived event.

This mapping was validated against `PKT_S2C_UpdateDeathTimer_s`: after normalization, all 69 packet rows match the ten players' final death total exactly.

## Data quality tiers

### Tier A — use in every analysis

`metadata.rawStats` has high-confidence final-match data for each player. The useful subset is:

- Identity and context: Riot ID, team, champion, lane, final level, items, runes and summoner spells.
- Combat: kills, deaths, assists, champion damage, damage taken, physical/magic/true damage split, self-mitigation, CC time, healing, shielding, time dead.
- Economy and farming: gold earned/spent, lane CS, neutral CS split by own/enemy jungle.
- Vision: vision score, wards placed, wards killed, control wards bought.
- Map pressure: turret/building/objective/epic-monster damage, turret and inhibitor credits.
- Objective attribution: dragon, Baron, Herald, Atakhan, steal and steal-assist credits. These are player credits; do **not** sum them as team objective counts until validated.
- Integrity: AFK, leaver and surrender flags.

The analyzer should derive compact comparisons from these values: per-minute rates, team damage/gold/vision shares, and same-role opponent deltas. This is substantially more useful than sending raw packet arrays.

### Tier B — use with explicit event confidence

- `PKT_S2C_UpdateDeathTimer_s`: reliable player death timestamps after `paramHint` normalization. It yields death sequences and 15–25 second multi-death windows, but does not yet identify killer or assists.
- `PKT_NPC_LevelUp_s`: player level-up timestamps and the new level value are structurally decoded. Use milestone timing (levels 6, 11, 16) when present; avoid assuming every level is present until more replays are validated.

### Tier C — retain locally, do not send to AI yet

- Item buy/sell packets have player identity and timestamps, but their item payload remains an unparsed tail. Use final items from Tier A; do not infer an item build order yet.
- Turret flags, snapshot packets, custom packet IDs and raw game-data packets lack enough semantic decoding to establish objective identity, killer, assists, gold swing, or position.

## Proposed compact digest v2

The model should receive a single normalized object, not raw ROFL artifacts.

```json
{
  "v": 2,
  "match": { "patch": "16.15", "duration_s": 2228, "winner": 100 },
  "teams": [
    { "id": 100, "k": 34, "g": 78610, "dmg": 186251, "vision": 247 },
    { "id": 200, "k": 35, "g": 73054, "dmg": 157048, "vision": 239 }
  ],
  "player_columns": ["id", "team", "lane", "champion", "k", "d", "a", "gold", "cs", "jcs", "dmg", "taken", "vision", "time_dead", "tower_dmg", "objective_dmg"],
  "players": [
    ["p0", 100, "TOP", "Yone", 10, 6, 11, 17037, 255, 24, 46216, 36118, 47, 262, 10222, 10222]
  ],
  "lane_deltas": [
    { "lane": "TOP", "left": "p0", "right": "p5", "gold": 3853, "cs": 32, "dmg": 16164 }
  ],
  "timeline": {
    "deaths": [[566.1, "p0"], [564.3, "p1"]],
    "level_milestones": [[...]],
    "death_windows": [{ "start": 554.6, "end": 573.8, "team100": ["p0", "p1"], "team200": ["p9"] }]
  },
  "quality": {
    "final_stats": "high",
    "death_timeline": "high",
    "level_timeline": "medium",
    "objective_timeline": "unavailable"
  }
}
```

Use short columnar arrays for repeated player metrics, with the `player_columns` legend once. This retains hundreds of meaningful values in a small token budget. The prompt should explicitly prohibit claims about killers, assists, objectives, or gold swings when the relevant tier is unavailable.

## Recommended implementation order

1. Expand the current digest from `rawStats` and calculate team/role comparisons.
2. Add `paramHint` normalization and Tier B death/level timelines with source confidence.
3. Add deterministic grouping for death windows; do not call them team fights unless enough players are involved.
4. Research semantic decoders for kills, objectives, structures and item payloads, then promote only validated fields.
5. Optional future enrichment: use Match-V5 timeline by game ID parsed from the ROFL file name. This can supply exact killer/assist/objective/gold timeline data, but must remain optional because ROFL-only analysis should work without an external Riot API call.

## Parser issue found

When the consuming project is ESM (`"type": "module"`), generated artifact `decoders.js` is interpreted as ESM while `rofl-parser` loads it via CommonJS `require()`. A local `.rofl-cache/package.json` declaring `type: commonjs` works around it for research. The library should generate `decoders.cjs` or write that artifact-local package boundary itself.
