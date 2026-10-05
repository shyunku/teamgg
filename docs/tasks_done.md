# team.gg 완료 작업

[tasks.md](tasks.md)에서 `🟢 DONE`으로 완료된 작업을 관리합니다. 문서 규칙과 컬럼은 `tasks.md`와 같습니다.

## 작업 목록

| Index | Tag | Updated | Status | Completed | Deps | 항목 | 완료 조건 |
|---:|---|---|---|---|---|---|---|
| [77](tasks/077.md) | unclassified | 2026-09-04 12:23 | 🟢 DONE | 2026-09-04 12:23 |  | 모노레포에 GPL-3.0 라이선스를 적용한다. | 루트 `LICENSE`가 GNU 공식 GPL v3 전문과 일치하고 README에 `GPL-3.0-only`와 파일 링크가 명시된다. |
| [76](tasks/076.md) | backend | 2026-10-03 16:03 | 🟢 DONE | 2026-10-03 16:03 | #66 | 최신 8개 패치 보존 정책을 운영자 수동 실행 없이 주기적으로 적용한다. | 7일 주기 systemd timer가 백엔드 무중단 online 모드로 정리하고, 중복 실행 방지·디스크/DB 부하 가드·work limit이 동작하며, 운영 실제 삭제·미완료 재개와 Discord 실패 알림이 검증된다. |
| [74](tasks/074.md) | backend | 2026-09-04 10:20 | 🟢 DONE | 2026-09-04 10:20 | #73 | legacy 문자열 키 `masteries` 테이블을 제거한다. | 런타임과 통계 trigger가 `masteries_numeric_v2` 전용으로 동작하고 legacy 테이블이 제거되며, 실제 갱신·주요 API 정상과 디스크 여유 증가(13GB → 40GB)가 확인된다. |
| [73](tasks/073.md) | backend | 2026-09-03 20:08 | 🟢 DONE | 2026-09-03 20:08 | #64 | 숙련도 쓰기를 `masteries_numeric_v2` 직접 쓰기로 전환한다. | 운영 읽기·쓰기가 numeric 저장소로 전환되고, 실제 Riot 갱신 표본의 양쪽 행·checksum 일치와 4시간 무오류 운영, backend healthy·주요 API 200이 확인된다. |
| 69 | backend | 2026-08-21 | 🟢 DONE | 2026-08-21 | #67 | DataExplorer DB 용량 임계값에 사람이 읽기 쉬운 용량 단위를 지원한다. | `DATA_EXPLORER_ALERT_DATABASE_BYTES`가 정수 바이트와 1024 기반 `K/M/G/T`(소수 포함)를 받고, 잘못된 값·범위 초과는 기본값으로 복귀하며, 단위 테스트와 백엔드 전체 테스트·빌드가 통과한다. |
| 68 | backend | 2026-08-23 17:04 | 🟢 DONE | 2026-08-23 17:04 | #67 | 별도 관리자 API 서버와 프론트엔드 `/admin` 운영 화면을 구현한다. | 관리자 서버가 DB 접속정보·JWT 키·Docker socket 없이 공유 비밀키 내부 API만 호출하고, 역할 검증·감사 로그·민감정보 마스킹이 적용되며, 백엔드·관리자 서버·프론트 빌드와 Compose config가 통과한다. |
| 67 | backend | 2026-08-21 | 🟢 DONE | 2026-08-21 |  | DataExplorer 운영 메트릭과 임계치 알림을 추가한다. | 5분 주기로 큐·일일 예산·DB 용량·순증가량을 `key=value` 로그로 남기고 임계치 최초 발생·정상화를 알리며, 일별 기준선 마이그레이션과 단위 테스트·전체 빌드가 통과한다. |
| [70](tasks/070.md) | backend | 2026-10-05 19:15 | 🟢 DONE | 2026-10-05 19:15 | #64, #74 | 기존 `match_participants` 행의 participant PK·match FK·summoner FK를 백엔드 무중단으로 채운다. | 전체 대상 숫자 키 NULL 0건, legacy와 숫자 관계 전체 일치, 오류·deadlock 없이 cursor 완료, 디스크 12GiB 안전선과 API health 유지, 처리량·binlog·디스크 증가량이 기록된다. |
| [66](tasks/066.md) | backend | 2026-09-02 22:34 | 🟢 DONE | 2026-09-02 22:34 | #58, #59 | 오래된 경기 데이터를 안전한 제한 배치로 삭제하는 보존 정책을 구현한다. | 최신 8개 패치를 보존하고 숫자 identity를 유지하는 bounded delete로 운영 48,351경기를 정리해 최종 dry-run 잔여 0건, backend healthy·주요 API 200이 확인된다. |
| [64](tasks/064.md) | backend | 2026-09-02 02:35 | 🟢 DONE | 2026-09-02 02:35 | #60 | 숫자 identity 기반과 소환사·경기 부모 키를 구축하고 숙련도 읽기를 숫자 저장소로 전환한다. | 부모 숫자 키 백필, 숙련도 compact shadow copy·checksum·동기화 trigger, 성능 비교를 거쳐 운영 `numeric_v2` 읽기가 전환되고 backend healthy·실제 API 200이 확인된다. |
| 63 | backend | 2026-08-31 14:59 | 🟢 DONE | 2026-08-31 14:59 |  | 운영 MySQL의 미사용 대형 인덱스를 정리한다. | 인덱스 크기·사용 통계·statement digest·코드·EXPLAIN 교차검증으로 확정한 약 8.1GiB 후보 4개가 제거되고, 대체 covering index 유지와 backend healthy·관련 API 200이 확인된다. |
| 62 | backend | 2026-08-31 16:09 | 🟢 DONE | 2026-08-31 16:09 |  | Champion Detail·메타 통계를 증분 집계로 전환한다. | 경기 10개 단위 증분 source·버전별 cursor·processed-match 마커 구조가 운영에 적용되고, 대상 버전 초기 백필과 집계·snapshot 생성 완료, API의 새 `updatedAt` 제공, fresh snapshot 재실행 skip이 확인된다. |
| 61 | backend | 2026-08-30 11:38 | 🟢 DONE | 2026-08-30 11:38 |  | 숙련도 통계를 dirty queue 기반 증분 집계로 전환한다. | 커버링 인덱스·dirty-champion queue·변경 trigger·materialized aggregate와 Top 30 조회가 운영 배포되고, 마이그레이션 clean, filesort 없는 계획, 갱신 시간과 snapshot 저장이 확인된다. |
| 60 | backend | 2026-08-21 | 🟢 DONE | 2026-08-21 |  | 스키마 버전 기록과 순차 마이그레이션 실행기를 도입한다. | 적용 이력·체크섬·dirty 상태를 기록하고 시작 시 드리프트를 검증하며, 명시적 `migrate`에서 `summoner_matches` 복합 PK를 재시작 가능하게 적용하고 전체 테스트·빌드가 통과한다. |
| 59 | backend | 2026-08-20 | 🟢 DONE | 2026-08-20 | #58 | DataExplorer 재처리 cooldown과 완료 작업 정리 정책을 구현한다. | 처리 상태와 재탐색 cooldown이 job과 분리되고, 완료 job·source만 기본 비활성 opt-in으로 재시작 가능하게 정리되며, cursor·bounded delete 테스트와 전체 테스트·빌드가 통과한다. |
| 58 | backend | 2026-08-20 | 🟢 DONE | 2026-08-20 |  | DataExplorer 탐색 범위를 제한한다. | DataExplorer와 관계 확장이 명시적 opt-in이 되고 깊이 0~10·일일 예산 경계와 재시작 경계가 적용되며, 관련 테스트와 전체 테스트·빌드가 통과한다. |
| 57 | replay | 2026-08-20 | 🟢 DONE | 2026-08-20 | #56 | 16.16 hero-death 패킷을 사망 victim fallback으로 연결한다. | 실제 `KR-8346113524.rofl`에서 kills/deaths 42건과 플레이어별 데스가 종료 통계와 전수 일치하고, typecheck·테스트·빌드가 통과한다. |
| 56 | replay | 2026-08-20 | 🟢 DONE | 2026-08-20 |  | `rofl-parser`를 갱신하고 샘플 리플레이를 AI 분석한다. | `rofl-parser@26.16.0`으로 `KR-8346113524.rofl`을 파싱·분석한 결과가 저장된다. |
| 55 | replay | 2026-08-08 | 🟢 DONE | 2026-08-08 |  | 리플레이 분석 서버 로그를 구조화한다. | Pino 로그에 ISO 시간·서비스 문맥이 붙고 직접·스트림·공유 분석의 시작·진행률·완료 시간·실패가 구조화되어 기록된다. |
| 54 | backend | 2026-08-08 | 🟢 DONE | 2026-08-08 | #37, #40 | Docker에서 백엔드 데이터 파일을 영속화한다. | 프로젝트 루트가 `/app`으로 고정되고 Data Dragon·통계 캐시가 `backend_datafiles` 볼륨에 유지된다. |
| 53 | backend | 2026-08-08 | 🟢 DONE | 2026-08-08 | #37, #40 | Docker 환경 주입 로그와 Redis 접속 문제를 진단한다. | Compose 환경 주입 로그가 명확해지고 원격 호스트 Redis 접속 거부 원인이 확인된다. |
| 43 | replay | 2026-08-20 | 🟢 DONE | 2026-08-20 | #6, #7 | 사건 dossier를 실제 OpenAI 분석 입력에 연결한다. | `06-event-dossiers.json`이 분석 입력으로 사용되어 `KR-8346113524.rofl`의 AI Markdown 결과가 생성된다. |
| 41 | replay | 2026-08-20 | 🟢 DONE | 2026-08-20 |  | `rofl-parser` 공식 타입과 bundled artifact를 검증한다. | `rofl-parser@26.16.1` 공식 타입으로 로컬 우회가 제거되고, ESM/CJS import와 실제 16.16 디코딩(unresolved payload 0), 테스트·빌드가 통과한다. |
| 40 | backend | — | 🟢 DONE |  |  | 백엔드 Dockerfile을 작성한다. | multi-stage Dockerfile과 `.dockerignore`로 백엔드 이미지가 빌드된다. |
| 39 | frontend | — | 🟢 DONE |  |  | 프론트엔드 Dockerfile을 작성한다. | 개발·프로덕션 multi-stage Dockerfile과 `.dockerignore`가 추가된다. |
| 38 | replay | — | 🟢 DONE |  |  | 리플레이 분석 서버 Dockerfile을 작성한다. | multi-stage Dockerfile과 `.dockerignore`가 추가된다. |
| 37 | frontend | — | 🟢 DONE |  | #35, #36, #38, #39, #40 | Docker Compose 실행 프로필을 구성한다. | 기본 실행은 서버 2개, `development` 프로필은 프론트엔드까지 3개가 실행된다. |
| 36 | backend | — | 🟢 DONE |  |  | 백엔드 Docker 환경 파일을 분리한다. | 백엔드와 분석 서버 환경 파일이 분리되어 DB·JWT 비밀값이 교차 노출되지 않는다. |
| 35 | replay | — | 🟢 DONE |  |  | 리플레이 Docker 환경 파일을 분리한다. | 분석 서버 환경 파일이 분리되어 OpenAI 비밀값이 교차 노출되지 않는다. |
| 34 | replay | — | 🟢 DONE |  | #37 | Compose 설정을 검증한다. | Compose 렌더링, 환경 격리, 분석 서버 production 시작 경로가 확인된다. |
| 33 | backend | — | 🟢 DONE |  |  | 공유 리플레이 분석 작업 저장과 조회 권한을 구현한다. | 공유 분석 작업이 MySQL에 저장되고 방장·참여자의 목록·진행률·결과 조회 권한이 분리된다. |
| 32 | backend | — | 🟢 DONE |  | #33 | 동일 내전의 중복 분석 실행을 차단한다. | 실행 중 작업이 내전당 하나로 제한되고 DB 행 잠금·상태 전이·request ID 검증으로 중복이 차단된다. |
| 31 | backend | — | 🟢 DONE |  | #32, #33 | 응답이 끊긴 분석 작업을 자동 실패 처리한다. | 오래된 작업이 기본 45분 후 실패 처리되어 새 업로드가 가능해진다. |
| 30 | frontend | — | 🟢 DONE |  | #24, #33 | ROFL을 브라우저에서 분석 서버로 직접 업로드한다. | 브라우저 업로드가 team.gg 백엔드를 거치지 않고 분석 서버로 전달된다. |
| 29 | replay | — | 🟢 DONE |  | #24 | 업로드 티켓을 헤더로 전달한다. | 티켓이 `X-Replay-Upload-Ticket` 헤더로 전달되고 CORS preflight 회귀 테스트가 통과한다. |
| 28 | replay | — | 🟢 DONE |  | #23 | 분석 진행 콜백을 제한하고 재시도한다. | 진행 콜백이 최대 초당 1회로 병합되고 완료·실패 콜백이 장시간 재시도된다. |
| 27 | backend | — | 🟢 DONE |  | #22, #31, #32, #33 | 공유 분석 연동 후 백엔드 회귀를 검증한다. | 백엔드 전체 테스트가 통과한다. |
| 26 | frontend | — | 🟢 DONE |  | #21, #30 | 공유 분석 연동 후 프론트 빌드를 검증한다. | 프론트엔드 프로덕션 빌드가 통과한다. |
| 25 | replay | — | 🟢 DONE |  | #24, #28, #29 | 공유 분석 연동 후 분석 서버를 검증한다. | 분석 서버 테스트·타입 검사·빌드가 통과한다. |
| 24 | replay | — | 🟢 DONE |  | #33 | 방장 서명 업로드 티켓을 검증한다. | 방장이 생성한 서명 티켓이 검증되고 해당 내전의 공유 분석 작업으로 직접 업로드된다. |
| 23 | replay | — | 🟢 DONE |  | #22, #33 | 분석 단계 진행률을 백엔드에 전달한다. | 업로드·디코딩·정제·프롬프트 구성·AI 분석 진행률이 최대 초당 1회 전달된다. |
| 22 | backend | — | 🟢 DONE |  | #33 | 분석 결과와 실패 원인을 저장한다. | 완료 Markdown·모델·실패 원인이 공유 분석 내역에 저장된다. |
| 21 | frontend | — | 🟢 DONE |  | #23, #30, #33 | 분석 진행률을 SSE UI에 연결한다. | 업로드 이후 서버 분석 단계가 SSE 전체 진행률로 정규화되어 클라이언트 진행 UI에 표시된다. |
| 20 | replay | — | 🟢 DONE |  |  | 샘플 ROFL을 디코딩한다. | `KR-8326522247.rofl`이 `rofl-parser@0.4.0`으로 디코딩된다. |
| 19 | replay | — | 🟢 DONE |  | #20 | 플레이어·팀 요약을 정제한다. | ROFL 종료 통계에서 플레이어와 팀 요약이 자동 생성된다. |
| 18 | replay | — | 🟢 DONE |  | #20 | 리플레이 내부 플레이어 ID를 부여한다. | `paramHint` 하위 16비트로 `p1`~`p10` ID가 부여된다. |
| 17 | replay | — | 🟢 DONE |  | #18, #19 | 정제 결과 스키마를 정의한다. | `refined/metadata.json`과 압축 테이블 형식의 정제 패킷이 생성된다. |
| 16 | replay | — | 🟢 DONE |  | #20 | 유효 패킷을 자동 선별한다. | parse level 0 초과이며 `namedParameters`가 있는 후보 패킷이 자동 보존된다. |
| 15 | replay | — | 🟢 DONE |  | #17, #18 | 게임플레이 이벤트를 정제한다. | 킬러·피해자, 이동, 스킬, 피해, 아이템, 와드, 체력, 구조물 이벤트가 정제된다. |
| 14 | replay | — | 🟢 DONE |  | #15 | 중립 오브젝트 이벤트를 정제한다. | 용·전령·바론 생성과 사망이 entity NetId로 연결되어 처치가 정제된다. |
| 13 | replay | — | 🟢 DONE |  | #17, #18 | 시야 이벤트를 정제한다. | 플레이어 시야 진입·이탈이 정제되고 사건별로 요약된다. |
| 12 | replay | — | 🟢 DONE |  | #14, #15 | 주요 사건 후보를 통합한다. | 킬·구조물·중립 오브젝트가 하나의 사건 후보 점수 체계로 병합된다. |
| 11 | replay | — | 🟢 DONE |  | #12, #13, #15 | 사건별 근거를 생성한다. | 사건 시작·종료 위치와 체력, 주변 인원, 와드, 시야, 피해, 스킬 근거가 생성된다. |
| 10 | replay | — | 🟢 DONE |  | #11, #12 | 사건 영향도와 신뢰도를 산정한다. | 영향도와 데이터·해석 신뢰도가 분리되어 계산된다. |
| 9 | replay | — | 🟢 DONE |  | #12 | 골드 역전 영향도를 계산한다. | 골드 시계열과 우위 역전 영향도 계산 로직이 구현된다. |
| 8 | replay | — | 🟢 DONE |  |  | 누락 데이터를 명시적으로 처리한다. | 골드·경험치 원천이 없으면 추정 대신 `unavailable`이 출력된다. |
| 7 | replay | — | 🟢 DONE |  | #6, #10, #11 | AI 입력 스키마 v3를 확정한다. | `teamgg-ai-event-input-v3` 입력이 `process/prompt-assets/06-event-dossiers.json`에 저장된다. |
| 6 | replay | — | 🟢 DONE |  | #11 | AI 분석 준비도를 검증한다. | 사건별 필수 근거 충족률과 전체 `readyForAi` 검증이 자동 생성된다. |
| 5 | replay | — | 🟢 DONE |  | #6, #7 | 샘플 사건 근거를 검증한다. | 샘플 5개 사건이 위치·체력·전투·시야 필수 검증을 모두 통과한다. |
| 4 | replay | — | 🟢 DONE |  | #7 | AI 프롬프트 크기를 줄인다. | 샘플 AI 입력이 minified 35,185자로 줄고 첫 실행 메타데이터 생성 순서가 검증된다. |
| 3 | replay | — | 🟢 DONE |  | #9, #10 | 사건 점수 계산 테스트를 추가한다. | 영향도와 골드 우위 역전 계산 단위 테스트가 통과한다. |
| 2 | replay | — | 🟢 DONE |  |  | 작업 artifact 보존 정책을 정한다. | 기본 분석 후 작업 폴더가 삭제되고 `--keep-artifacts`/`keepArtifacts=true`일 때만 보존된다. |
| 1 | replay | — | 🟢 DONE |  |  | SSE 응답에 CORS 헤더를 추가한다. | Fastify 생명주기를 우회하는 SSE 응답에도 허용된 Origin 헤더가 붙는다. |

## 검증 기록

- 2026-09-02 02:41 — Task 64의 범위를 숫자 identity 기반, 소환사·경기 부모 키, 숙련도 numeric 읽기 전환까지로 확정해 DONE 처리했다. 운영 `numeric_v2` 전환, 결과 일치, 직접 SQL 성능, backend healthy와 실제 API 200을 검증했다. Participant·하위 관계·숫자 FK/읽기 전환·legacy 제거는 독립적인 완료 조건을 가진 #70~#75로 분리했다.

- 2026-08-31 16:09 — Task 62 운영 상태를 재확인했다. `champion_detail_statistics_progress`의 대상 full version 6개가 모두 `completed=1`이고 처리 경기 합계는 500,470건이다. 새 champion·meta-summary snapshot의 `updatedAt`은 `2026-08-30T18:30:56.081618598Z`로 일치했으며 두 API 모두 이를 정상 제공한다. 현재 loop가 fresh shared snapshot을 감지해 다음 만료까지 skip하는 것도 확인했다. 남아 있던 전체 초기 백필, 최종 집계, snapshot/API 갱신, 재실행 skip 완료 조건을 충족해 Task 62를 DONE 처리했다.

- 2026-08-31 16:03 — Task 64의 운영 foundation `20260830_005`와 호환성 보완 `20260831_006`을 적용했다. `006`은 33ms, `dirty=0`으로 완료됐으며 참가자 트리거가 기존 저장 순서에서도 match·summoner numeric identity를 선점하고 거부 `SIGNAL`을 사용하지 않음을 확인했다. 첫 배포에서 발견한 strict DAO scan 오류와 participant parent 오류를 수정한 뒤 영향받은 경기 작업 20건이 모두 자동 복구되어 `done`, `last_error=NULL`, 경기 및 참가자 저장 완료 상태가 됐다. 두 차례 제한 백필로 소환사 221,500건을 처리했고 cursor가 저장됐다. 종료 시 backend·replay healthy, 루트·champion·meta-summary API 정상, 관련 최근 오류 없음, `Threads_running=3`, `Innodb_row_lock_current_waits=0`, 루트 디스크 20GB 여유를 확인했다. 디스크 여유와 서비스 부하를 고려해 추가 연속 백필은 실행하지 않았으며 Task 64는 WIP로 유지한다.

- 2026-08-31 14:59 — Task 63의 `20260830_004`을 운영 MySQL 8.0.45에 적용했다. 1.519초에 `dirty=0`으로 완료됐고 제거 대상 인덱스 4개가 모두 부재하며 `masteries_champion_points_level_covering_index(champion_id, champion_points, champion_level)`가 유지됨을 확인했다. backend 재기동 후 healthy, 루트 응답, champion·meta-summary API 200 및 최근 migration/deadlock/duplicate/temporary-file 오류 부재를 검증했다.

- 2026-08-30 22:51 — Task 64 1단계에서 숫자 키 매핑·진행 테이블, 부모 3개 테이블의 additive nullable 키, 신규 쓰기 동기화 트리거와 명시적 `backfill-numeric-keys` 명령을 구현했다. 환경변수 경계 단위 테스트와 백엔드 전체 테스트·빌드를 통과했고, 임시 MySQL 8 데이터베이스에서 레거시 행 백필, 신규 행 즉시 키 할당, 부모 없는 참가자 거부, 반복 실행 시 0건 처리를 검증했다. #62가 진행 중이므로 운영 마이그레이션과 백필은 실행하지 않았고 Task 64는 WIP로 유지한다.

- 2026-08-30 05:39 — Task 61·62 운영 배포 전 기준값을 읽기 전용으로 측정했다. `masteries`는 추정 35,330,564행(데이터 약 10.0GiB, 인덱스 약 6.0GiB), `match_participants`는 추정 11,276,579행이었다. 기존 Champion Detail 첫 집계는 확인 종료 시점에도 546초째 실행 중이었고, 측정 구간의 DB 전체 임시 테이블은 6개(디스크 3개), 행 잠금 대기는 12회·약 87.7초 증가했다. 동시 DataExplorer 쓰기가 있어 잠금 증가분 전체를 통계 작업에 귀속하지 않는다. 마지막 성공 스냅샷은 Champion Detail `2026-07-30 17:54:40`, Mastery `2026-08-12 11:18:15`였으며 운영 서비스·DB·파일은 변경하지 않았다.

- 2026-08-30 04:55 — Task 62에서 최근 패치 선필터와 단일 champion_detail_statistics_source staging 테이블, CTE 기반 메타·카운터 집계, 단계별 실행 시간 로그, 마이그레이션·운영 검증 SQL을 추가했다. 구조 단위 테스트, 백엔드 전체 테스트·빌드와 격리 MySQL 8.0 통합 테스트를 통과했으며 운영 규모의 시간·임시 공간·잠금 비교 전까지 WIP로 유지한다.

- 2026-08-30 04:33 — Task 61에서 숙련도 통계를 챔피언별 dirty queue 기반 증분 집계로 전환하고, 온라인 커버링 인덱스·변경 트리거·동시 갱신 보존 cutoff·materialized 조회·운영 검증 SQL을 추가했다. 관련 패키지 테스트를 통과했으며 운영 규모 DB의 정확도·실행 시간·잠금·쿼리 계획 검증 전까지 WIP로 유지한다.

- 2026-08-23 17:04 — Task 68에서 별도 관리자 API 서버, 기존 프론트엔드 관리자 화면, 백엔드 역할·감사·운영 이벤트 스키마와 내부 API, Compose 배포 구성을 구현했다. 백엔드 `go test ./...`·`go build ./...`, 관리자 서버 테스트와 Windows/Linux 빌드, 프론트 `npm run build`, 예제 환경파일 기반 `docker compose config`, `git diff --check`를 통과했다. Docker 데몬 비실행으로 이미지 빌드는 수행하지 못했다.

- 2026-08-21 — Task 69에서 `DATA_EXPLORER_ALERT_DATABASE_BYTES`에 기존 정수 바이트와 1024 기반 `K/KB/M/MB/G/GB/T/TB` 및 소수 단위를 지원하고, 유효값·잘못된 값·범위 초과 테스트와 전체 `go test ./...`, `go build ./...`, `git diff --check`를 통과했다.
- 2026-08-21 — Task 67에서 DataExplorer 일일 사용량·큐·추정 테이블 성장·DB/임시 공간 메트릭과 중복 억제 임계치 알림을 구현했다. `20260821_001` 일별 기준선 마이그레이션과 운영 예산 문서를 추가했으며 `go test ./...`, `go build ./...`, `git diff --check`를 통과했다. 실제 DB에는 실행하지 않았다.
- 2026-08-21 — Task 60에서 9개 기존 SQL 이력을 체크섬·dirty 상태와 함께 `schema_migrations`에 기록하는 순차 실행기를 도입했다. 일반 시작은 기본 `validate`, 명시적 `migrate` 명령만 `up`으로 동작하며, `summoner_matches` 복합 PK 백필은 미러 트리거와 DB cursor로 재시작 가능하다. `go test ./...`, `go build ./...`를 통과했고 운영 DB에는 실행하지 않았다.
- 2026-08-20 — Task 59에서 DataExplorer 처리 상태와 재탐색 cooldown을 job과 분리하고 레거시 상태 백필 및 완료 job/source의 재시작 가능한 소량 정리를 구현했다. 삭제는 기본 비활성으로 두었고 `go test ./...`, `go build ./...`, `git diff --check`를 통과했다.
- 2026-08-20 — Task 58에서 DataExplorer 무제한 관계 확장을 제거하고 안전한 opt-in 기본값, 깊이·예산 경계 및 재시작 동작을 구현했으며 `go test ./...`와 `go build ./...`를 통과했다.
- 2026-08-20 — `rofl-parser@26.16.1` 공식 타입·ESM/CJS exports와 bundled 16.16 artifact를 실제 리플레이로 검증하고 Task 41을 완료했다.
- 2026-08-20 — `docs/reports/2026-08-20-production-schema-review.md`의 운영 DB 용량·수집 구조 분석을 중복 제거해 backend 후속 작업 58~67로 분리했다.
- 2026-08-20 — 백엔드·프론트엔드·리플레이 작업을 루트 표 하나로 통합했다.
- 기존 리플레이 문서의 TODO 10개는 최신 루트 보드의 같은 항목과 중복되어 한 번만 유지했다.
- 여러 프로젝트가 포함된 완료 항목은 단일 태그 규칙을 지키기 위해 앱별 행으로 분리했다.
- 기존 문서에 완료 날짜가 없던 이력은 추정하지 않고 `—`로 유지했다.
