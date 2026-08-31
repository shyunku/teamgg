# 운영 인덱스 사용량 및 중복 검증 보고서

- 검증 일시: 2026-08-30 21:52 ~ 22:03 KST
- 대상: backend Task #63
- 운영 환경: MySQL 8.0.45, `performance_schema=ON`
- 운영 변경 여부: 없음. 모든 운영 조회는 읽기 전용으로 수행했다.

## 1. 판정 기준

운영 MySQL의 uptime은 검증 시점 기준 45,813초(약 12.7시간)였다. 따라서
`performance_schema`의 0회 사용만으로 인덱스를 제거하지 않고 다음 근거를 함께
교차검증했다.

1. `information_schema.statistics`의 실제 컬럼 순서와 cardinality
2. `mysql.innodb_index_stats`의 실제 인덱스 크기
3. `performance_schema.table_io_waits_summary_by_index_usage`
4. `sys.schema_unused_indexes`와 `sys.schema_redundant_indexes`
5. 정규화된 statement digest
6. 현재 백엔드 SQL의 정적 사용 경로
7. 후보 인덱스를 제외한 `EXPLAIN FORMAT=TREE`

## 2. 제거 확정 후보

| 테이블·인덱스 | 크기 | 관찰된 읽기 | 판정 |
| --- | ---: | ---: | --- |
| `match_participant_perk_styles_description_index` | 1,404.8MB | 0 | `description`은 검색 조건이 아니라 CASE 분류값으로만 사용한다. 실제 룬 조인은 `match_participant_id` 인덱스를 사용한다. |
| `masteries_champion_id_champion_points_index` | 6,168.0MB | 39,858,589 | 신규 `(champion_id, champion_points, champion_level)` covering index의 완전한 prefix 중복이다. 읽기 대부분은 신규 인덱스 생성 당시의 1회성 dirty queue 초기화였다. 현재 동일 쿼리 EXPLAIN은 신규 covering index를 선택한다. |
| `match_participants_participant_id_index` | 319.0MB | 0 | `participant_id` 단독 조회가 없다. DataExplorer bootstrap은 `(match_id, participant_id)` PK range scan을 사용한다. |
| `match_participants_team_position_index` | 419.9MB | 0 | 최근 경기로 먼저 제한한 뒤 `team_position != ''''`를 검사한다. 후보 인덱스를 제외한 EXPLAIN도 `(match_id, participant_id)` PK를 사용한다. |

예상 회수 인덱스 공간은 합계 약 **8,312.7MB(약 8.1GiB)**다. 실제 파일시스템
회수 시점과 크기는 InnoDB의 테이블스페이스 설정 및 purge 상태에 따라 달라질 수 있다.

## 3. 유지한 인덱스

| 인덱스 | 크기 | 유지 이유 |
| --- | ---: | --- |
| `match_participants_champion_id_index` | 374.9MB | 챔피언 단위 집계·조회 경로를 보존한다. 짧은 uptime의 0회 관찰만으로 제거하지 않는다. |
| `match_participants_match_participant_id_index` | 813.8MB | 상세·룬 테이블 조인과 참가자 ID 조회에 필요하다. |
| `match_participants_summoner_puuid_fk` | 1,409.8MB | 소환사 최근 경기 조회가 `puuid`로 참가자를 찾는다. |
| `match_participant_perk_styles_id_fk` | 2,272.8MB | 증분 Champion Detail source의 룬 조인에서 실제로 사용됐다. |
| `match_participant_perk_style_selection_id_fk` | 4,134.7MB | 룬 selection 조인에서 실제로 사용됐다. |

## 4. EXPLAIN 결과

- 숙련도 챔피언 목록: 기존 중복 인덱스를 제외해도 신규 covering index의
  covering skip scan을 사용했다.
- 숙련도 집계: 신규 covering index에서 `champion_id` lookup을 수행했다.
- DataExplorer bootstrap: 저선택도 후보 인덱스 없이 복합 PK range scan을 사용했다.
- 최근 경기 참가자: `team_position` 인덱스 없이 `match_id` PK range scan을 사용했다.
- 룬 source 조인: `description` 인덱스 없이
  `match_participant_perk_styles_id_fk(match_participant_id)`와
  selection의 `style_id` 인덱스를 사용했다.

## 5. 구현

- 마이그레이션 `20260830_004`을 추가했다.
- 삭제 전 인덱스 이름뿐 아니라 컬럼 순서까지 확인한다.
- 같은 이름의 인덱스가 예상과 다른 컬럼을 가지면 삭제하지 않고 실패한다.
- 이미 삭제된 인덱스는 건너뛰므로 중간 실패 후 재실행할 수 있다.
- 각 DDL은 `ALGORITHM=INPLACE, LOCK=NONE`을 명시한다.
- 현재 기준 스키마에서도 제거 대상 인덱스를 제외했다.
- 수동 복구용
  `apps/backend/scripts/rollback-20260830-remove-unused-indexes.sql`을 추가했다.

## 6. 검증

- 백엔드 `go test ./...` 통과
- 백엔드 `go build ./...` 통과
- 격리 MySQL 8.0에서 제거 SQL 실행 후 대상 인덱스 0개 확인
- 제거 후 기존 데이터 조회 및 신규 covering index 조회 성공
- rollback SQL 실행 후 제거 대상 4개 인덱스 재생성 확인
- `git diff --check` 통과

## 7. 운영 적용 전 남은 조건

운영 DB에는 아직 `20260830_004`를 적용하지 않았다. 특히 6.17GB 숙련도 인덱스
제거와 현재 #62의 `match_participants` 백필이 동시에 수행되면 I/O 경쟁과 짧은
metadata lock 대기가 발생할 수 있다.

2026-08-30 22:04 KST 사전 점검에서는 InnoDB 트랜잭션 1개·최대 1초였고 10초 이상
실행 중인 사용자 쿼리는 없었다. 루트 볼륨은 128GB 중 101GB 사용, 28GB 여유였다.
다만 #62는 `16.15.802.4387`, cursor `KR_8334105753`에서 약 721~732경기/분
속도로 계속 진행 중이었다.

다음 조건을 충족한 저부하 창에서만 적용한다.

1. #62 백필 상태와 장기 실행 트랜잭션을 확인한다.
2. backend를 안전하게 중지하거나 통계 작업을 일시 중단한다.
3. `docker compose run --rm backend migrate`로 마이그레이션한다.
4. 대상 인덱스 부재, 마이그레이션 clean, backend health를 확인한다.
5. 주요 EXPLAIN과 #62 cursor 재개를 다시 확인한다.
6. 회귀가 확인되면 rollback SQL로 필요한 인덱스만 재생성한다.

운영 적용과 사후 검증이 끝나기 전까지 Task #63은 WIP로 유지한다.
