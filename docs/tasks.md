# team.gg 통합 작업 보드

이 문서는 `backend`, `frontend`, `replay` 세 프로젝트의 모든 작업을 관리하는 유일한 작업 보드입니다.

## 관리 규칙

- 새 작업은 `🔴 TODO`로 등록합니다.
- 실제 구현을 시작하기 직전에 `🟡 WIP`로 변경합니다.
- 구현과 검증이 모두 끝난 작업만 `🔵 DONE`으로 변경합니다.
- 동시에 실제 코드 구현 묶음 하나만 `WIP`로 두되, 배포·운영 규모 검증 대기 항목은 병행할 수 있습니다.
- `Tag`는 `backend`, `frontend`, `replay` 중 하나만 사용합니다.
- `Date`는 TODO/WIP의 등록 또는 갱신일, DONE의 완료일을 `YYYY-MM-DD`로 기록합니다.
- 기존 문서에 날짜가 없던 이력은 `—`로 유지합니다.
- 가장 큰 Index가 위에 오도록 정렬하며 새 작업은 현재 최댓값에 1을 더합니다.
- 선행 작업이 반드시 완료되어야 하는 작업은 `Dependencies`에 `#41` 또는 `#41, #5` 형식으로 기록하며, 하드 의존성이 없으면 `—`로 표시합니다.
- `추천 작업 순서`에는 미완료 작업 중 다음에 진행할 작업을 의존성·운영 위험도·효과를 기준으로 최대 20개까지 나열하며, 작업 추가 및 상태·의존성 변경 시 함께 갱신합니다.
- 세 앱의 신규 작업과 상태 변경은 이 문서에서만 관리합니다.

## 추천 작업 순서

`#68, #61, #62, #63, #64, #65, #66, #42, #51, #50, #52, #49, #44, #45, #46, #47, #48`

## 작업 목록

| Index | Tag | Status | Date | Dependencies | Description |
|---:|---|---|---|---|---|
| 69 | backend | 🔵 DONE | 2026-08-21 | #67 | `DATA_EXPLORER_ALERT_DATABASE_BYTES`가 기존 정수 바이트 값과 함께 `5G`, `200M`, `1.5TB` 같은 사람이 읽기 쉬운 1024 기반 용량 단위를 지원하도록 파서·테스트·운영 문서를 개선했다. 잘못된 값과 범위 초과 값은 안전하게 기본값으로 복귀하며, 단위 테스트와 백엔드 전체 테스트·빌드를 통과했다. |
| 68 | backend | 🔴 TODO | 2026-08-21 | #67 | 운영 로그·메트릭·서비스 상태·작업 큐·오류 현황 등을 인증된 관리자만 조회할 수 있는 관리 사이트를 설계하고 구현한다. 관리 API 서버만 별도로 두고 기존 team.gg 프론트엔드에 관리자 화면을 추가하는 방식과, 외부 노출·권한·배포 경계를 분리한 독립 Admin 서버+프론트엔드 방식의 보안성·운영 복잡도·개발 비용을 비교해 아키텍처를 확정하며, 관리자 인증·역할 기반 권한·감사 로그·민감정보 마스킹·조회 범위 제한을 포함한다. |
| 67 | backend | 🔵 DONE | 2026-08-21 | — | DataExplorer에 5분 기본 저빈도 운영 메트릭 워커를 추가했다. 대규모 테이블을 전체 스캔하지 않고 `information_schema` 추정 행 수와 일별 DB 기준선으로 소환사·경기·숙련도 순증가량을 계산하며, 큐·일일 예산·DB 용량·임시 테이블 상태를 parseable `key=value` 로그로 노출한다. 환경변수 임계치의 최초 발생·정상화 알림, 일별 기준선 마이그레이션, 운영 예산 가이드와 단위 테스트를 추가했고 전체 Go 테스트·빌드를 통과했다. 실제 DB 마이그레이션은 실행하지 않았다. |
| 66 | backend | 🔴 TODO | 2026-08-20 | #58, #59 | 경기·참가자·숙련도 데이터의 보존 기간과 아카이브 정책을 설계하고, 작은 배치·재시작 가능 cursor·실행 제한·dry-run을 갖춘 정리 작업을 구현한다. |
| 65 | backend | 🔴 TODO | 2026-08-20 | #60, #64 | 룬 데이터를 참가자당 고정 컬럼 또는 단일 `participant_perks` 행으로 평탄화하는 스키마를 설계하고, 이중 쓰기·백필·검증·읽기 전환·구 테이블 제거 순서의 무중단 마이그레이션을 구현한다. |
| 64 | backend | 🔴 TODO | 2026-08-20 | #60 | `summoners`, `matches`, `match_participants`에 숫자 PK를 도입하고 PUUID·match ID·UUID가 하위 테이블과 보조 인덱스에 반복되지 않도록 숫자 FK 기반 스키마 v2 및 단계적 백필·전환 계획을 구현한다. |
| 63 | backend | 🔴 TODO | 2026-08-20 | — | 운영 DB의 `performance_schema` 사용량과 실제 통계/조회 쿼리 `EXPLAIN`을 다시 수집해 미사용·중복 인덱스를 확정하고, 우선 `match_participant_perk_styles(description)` 등 후보를 롤백 가능한 마이그레이션으로 제거한다. |
| 62 | backend | 🟡 WIP | 2026-08-30 05:39 | — | 최근 패치 선필터 후 참가자·룬·아이템·동일 라인 상대를 한 행으로 정규화하는 영구 staging 테이블과 CTE 메타·카운터 집계를 구현해 기존 24개 명시적 임시 테이블을 제거했다. 구조 테스트, 전체 Go 테스트·빌드, 격리 MySQL 8 통합 테스트에서 패치 필터·메타·카운터 결과를 검증했다. 운영 구버전 기준으로 첫 Champion Detail 집계가 9분 6초 이상 전역 통계 락을 점유했고 과거 중복 인덱스 오류가 확인됐다. 배포 후 실행 시간·임시 공간·잠금을 동일 기준으로 비교한 뒤 완료 처리한다. |
| 61 | backend | 🟡 WIP | 2026-08-30 05:39 | — | 커버링 인덱스, dirty-champion queue·변경 트리거, DB cutoff 기반 재시작·동시 갱신 안전 처리, materialized aggregate 및 챔피언별 Top 30 조회 전환을 구현하고 단위 테스트를 통과했다. 운영 구버전 `EXPLAIN`에서 약 3,533만 mastery 행 전체 인덱스 순회와 대표 챔피언 Top 30의 약 42만 행 범위를 확인했다. 배포 후 정확도·실행 시간·잠금·쿼리 계획을 동일 기준으로 비교한 뒤 완료 처리한다. |
| 60 | backend | 🔵 DONE | 2026-08-21 | — | 적용된 마이그레이션을 기록하는 스키마 버전 테이블과 순차 실행기를 도입했다. 시작 시 체크섬·dirty 상태·필수 테이블/컬럼/인덱스 드리프트를 검증하고, 명시적 `migrate` 명령에서 누락된 `summoner_matches(puuid, match_id)` PK를 미러 트리거·재시작 가능한 keyset cursor·중복 제거·원자적 교체로 안전하게 적용한다. 전체 Go 테스트와 빌드를 통과했으며 실제 운영 DB에는 실행하지 않았다. |
| 59 | backend | 🔵 DONE | 2026-08-20 | #58 | 소환사·경기별 `last_processed_at`/`next_eligible_at` 상태를 분리해 cooldown 중 재등록을 차단하고, 기존 완료 job을 PK cursor로 재시작 가능하게 소량 백필한다. 삭제는 기본 비활성 opt-in이며 상태가 보존된 `done` job만 보존 기간 후 제한된 배치로 정리한다. `match_sources`도 별도 복합 PK cursor로 스캔·삭제하고 pending/processing/failed 행은 보존한다. 환경변수 안전 경계, cursor 재시작·진행, bounded delete 테스트와 전체 Go 테스트·빌드를 통과했다. |
| 58 | backend | 🔵 DONE | 2026-08-20 | — | DataExplorer를 명시적 opt-in으로 전환하고 기존 참가자 bootstrap과 참가자 관계 확장을 별도 opt-in으로 분리했다. `bounded` 정책과 `maxDepth` 0~10 경계, 신규 소환사/경기 일일 예산 500/1500 기본값, 저장된 작업 깊이를 유지하는 재시작 경계를 적용했으며 공용 매치 저장 경로의 무조건 재탐색을 제거했다. 환경변수·깊이·예산·재시작 테스트와 전체 Go 테스트·빌드를 통과했다. |
| 57 | replay | 🔵 DONE | 2026-08-20 | #56 | `rofl-parser@26.16.0`의 16.16 hero-death packet param을 종료 통계와 교차 검증하는 victim fallback으로 연결하고 진행률을 단조 증가시켰다. 실제 `KR-8346113524.rofl`에서 kills/deaths 42건, dossier 5건, 플레이어별 데스 전수 일치 및 typecheck/test 13건/build 통과. |
| 56 | replay | 🔵 DONE | 2026-08-20 | — | `rofl-parser`를 `26.16.0`으로 갱신하고 `KR-8346113524.rofl`을 파싱·AI 분석해 `tmp/analysis/KR-8346113524-analysis.md`에 저장했다. |
| 55 | replay | 🔵 DONE | 2026-08-08 | — | Pino 로그에 ISO 시간과 서비스 문맥을 추가하고 직접·스트림·공유 분석의 시작, 제한된 진행률, 완료 시간, 실패를 구조화했다. |
| 54 | backend | 🔵 DONE | 2026-08-08 | #37, #40 | Docker에서 프로젝트 루트를 `/app`으로 고정해 Data Dragon과 통계 캐시를 `backend_datafiles` 볼륨에 영속화했다. |
| 53 | backend | 🔵 DONE | 2026-08-08 | #37, #40 | Docker Compose 프로세스 환경 주입 로그를 명확히 하고 원격 호스트 Redis 접속 거부 원인을 진단했다. |
| 52 | frontend | 🔴 TODO | 2026-08-08 | #37, #39 | Docker Desktop 환경에서 development 프로필의 프론트엔드 빌드·기동·기본 화면 smoke test를 수행한다. |
| 51 | backend | 🔴 TODO | 2026-08-08 | #37, #40 | Docker Desktop 환경에서 백엔드 빌드·기동·health/API smoke test를 수행한다. |
| 50 | replay | 🔴 TODO | 2026-08-08 | #37, #38 | Docker Desktop 환경에서 리플레이 분석 서버 빌드·기동·health smoke test를 수행한다. |
| 49 | replay | 🔴 TODO | 2026-08-08 | #22, #24, #28, #30, #33 | 배포 환경에서 방장 업로드·삭제, 참여자 진행률·결과 조회, 권한 거부, 중복·만료·재사용 티켓과 중단 복구를 수동 통합 검증한다. |
| 48 | replay | 🔴 TODO | 2026-08-08 | #9 | 시간대별 개인·팀 골드와 경험치가 제공되면 팀 골드 곡선과 우위 역전을 실제 사건 영향도에 연결한다. |
| 47 | replay | 🔴 TODO | 2026-08-08 | #14 | 중립 오브젝트 스틸 여부를 확정할 수 있도록 팀 정보와 강타 근거를 사건에 추가한다. |
| 46 | replay | 🔴 TODO | 2026-08-08 | #13, #15 | `detectorWardNetId`를 시야 진입·이탈 이벤트와 실제 와드에 연결한다. |
| 45 | replay | 🔴 TODO | 2026-08-08 | #11, #15 | 10초 이동 샘플보다 정밀한 합류 시간과 4대5 상태 계산을 추가한다. |
| 44 | replay | 🔴 TODO | 2026-08-08 | #7, #43 | 최종 AI 출력용 대안 행동과 개인 피드백 응답 스키마를 구현한다. |
| 43 | replay | 🔵 DONE | 2026-08-20 | #6, #7 | `06-event-dossiers.json`을 실제 OpenAI 분석 입력에 연결하고 `KR-8346113524.rofl`의 AI Markdown 분석 결과 생성을 검증했다. |
| 42 | replay | 🔴 TODO | 2026-08-20 | #17, #20, #41 | 보존된 실제 디코딩 결과를 사용하는 fixture 기반 통합 테스트를 추가한다. |
| 41 | replay | 🔵 DONE | 2026-08-20 | — | `rofl-parser@26.16.1`로 갱신하고 로컬 타입 우회를 제거했다. 공식 타입 기반 typecheck, ESM import/CommonJS require, bundled artifact 실제 16.16 디코딩(214 files, 857,724 packets, unresolved payload 0), 테스트 13건과 빌드를 검증했다. |
| 40 | backend | 🔵 DONE | — | — | 백엔드용 multi-stage Dockerfile과 `.dockerignore`를 추가했다. |
| 39 | frontend | 🔵 DONE | — | — | 프론트엔드용 개발·프로덕션 multi-stage Dockerfile과 `.dockerignore`를 추가했다. |
| 38 | replay | 🔵 DONE | — | — | 리플레이 분석 서버용 multi-stage Dockerfile과 `.dockerignore`를 추가했다. |
| 37 | frontend | 🔵 DONE | — | #35, #36, #38, #39, #40 | 기본 실행은 서버 2개, `development` 프로필은 프론트엔드까지 3개가 실행되는 루트 Docker Compose를 구성했다. |
| 36 | backend | 🔵 DONE | — | — | 백엔드 Docker 환경 파일을 분석 서버와 분리해 DB·JWT 비밀값의 교차 노출을 방지했다. |
| 35 | replay | 🔵 DONE | — | — | 분석 서버 Docker 환경 파일을 백엔드와 분리해 OpenAI 비밀값의 교차 노출을 방지했다. |
| 34 | replay | 🔵 DONE | — | #37 | Compose 렌더링과 환경 격리, 분석 서버 production 시작 경로를 검증했다. |
| 33 | backend | 🔵 DONE | — | — | 공유 리플레이 분석 작업을 MySQL에 저장하고 방장·참여자의 목록, 진행률, Markdown 결과 조회 권한을 분리했다. |
| 32 | backend | 🔵 DONE | — | #33 | 동일 내전의 실행 중 작업을 하나로 제한하고 DB 행 잠금·상태 전이·request ID 검증으로 중복 분석을 차단했다. |
| 31 | backend | 🔵 DONE | — | #32, #33 | 분석 서버 응답이 끊긴 오래된 작업을 기본 45분 후 실패 처리해 새 업로드가 가능하도록 했다. |
| 30 | frontend | 🔵 DONE | — | #24, #33 | 브라우저가 ROFL을 team.gg 백엔드를 거치지 않고 분석 서버로 직접 업로드하도록 연결했다. |
| 29 | replay | 🔵 DONE | — | #24 | 업로드 티켓을 URL 대신 `X-Replay-Upload-Ticket` 헤더로 전달하고 CORS preflight 회귀 테스트를 추가했다. |
| 28 | replay | 🔵 DONE | — | #23 | 분석 진행 콜백을 최대 초당 1회로 병합하고 완료·실패 콜백을 장시간 재시도하도록 보강했다. |
| 27 | backend | 🔵 DONE | — | #22, #31, #32, #33 | 공유 분석 연동 후 백엔드 전체 테스트를 통과했다. |
| 26 | frontend | 🔵 DONE | — | #21, #30 | 공유 분석 연동 후 프론트엔드 프로덕션 빌드를 통과했다. |
| 25 | replay | 🔵 DONE | — | #24, #28, #29 | 공유 분석 연동 후 분석 서버 테스트·타입 검사·빌드를 통과했다. |
| 24 | replay | 🔵 DONE | — | #33 | team.gg 방장이 생성한 서명 업로드 티켓을 검증하고 해당 내전의 공유 분석 작업으로 직접 업로드하도록 구현했다. |
| 23 | replay | 🔵 DONE | — | #22, #33 | 업로드·디코딩·정제·프롬프트 구성·AI 분석 진행률을 backend에 최대 초당 1회 전달하도록 구현했다. |
| 22 | backend | 🔵 DONE | — | #33 | 완료 Markdown과 모델, 실패 원인을 공유 분석 내역에 저장하도록 구현했다. |
| 21 | frontend | 🔵 DONE | — | #23, #30, #33 | 업로드 이후 서버 분석 단계를 SSE 전체 진행률로 정규화해 클라이언트 진행 UI에 연결했다. |
| 20 | replay | 🔵 DONE | — | — | `KR-8326522247.rofl`을 `rofl-parser@0.4.0`으로 디코딩했다. |
| 19 | replay | 🔵 DONE | — | #20 | ROFL 종료 통계에서 플레이어와 팀 요약을 자동 정제했다. |
| 18 | replay | 🔵 DONE | — | #20 | `paramHint` 하위 16비트로 `p1`부터 `p10`까지 리플레이 내부 ID를 부여했다. |
| 17 | replay | 🔵 DONE | — | #18, #19 | `refined/metadata.json`과 압축 테이블 형식의 정제 패킷을 생성했다. |
| 16 | replay | 🔵 DONE | — | #20 | 모든 패킷을 순회해 parse level 0 초과 및 `namedParameters`가 있는 후보를 자동 보존했다. |
| 15 | replay | 🔵 DONE | — | #17, #18 | 킬러·피해자, 이동, 스킬, 피해, 아이템, 와드, 체력, 구조물 이벤트를 정제했다. |
| 14 | replay | 🔵 DONE | — | #15 | 용·전령·바론 생성과 사망을 entity NetId로 연결해 중립 오브젝트 처치를 정제했다. |
| 13 | replay | 🔵 DONE | — | #17, #18 | 플레이어 시야 진입·이탈을 정제하고 사건별로 요약했다. |
| 12 | replay | 🔵 DONE | — | #14, #15 | 킬·구조물·중립 오브젝트를 하나의 사건 후보 점수 체계로 병합했다. |
| 11 | replay | 🔵 DONE | — | #12, #13, #15 | 사건 시작·종료 위치와 체력, 주변 인원, 와드, 시야, 피해, 스킬 근거를 생성했다. |
| 10 | replay | 🔵 DONE | — | #11, #12 | 사건 영향도와 데이터·해석 신뢰도를 분리해 계산했다. |
| 9 | replay | 🔵 DONE | — | #12 | 골드 시계열과 우위 역전의 영향도 계산 로직을 구현했다. |
| 8 | replay | 🔵 DONE | — | — | 골드·경험치 원천이 없을 때 가짜 추정 대신 명시적인 `unavailable`을 출력했다. |
| 7 | replay | 🔵 DONE | — | #6, #10, #11 | AI 직전 입력을 `teamgg-ai-event-input-v3`으로 확정하고 `process/prompt-assets/06-event-dossiers.json`에 저장했다. |
| 6 | replay | 🔵 DONE | — | #11 | 사건별 필수 근거 충족률과 전체 `readyForAi` 검증을 자동 생성했다. |
| 5 | replay | 🔵 DONE | — | #6, #7 | 샘플의 5개 사건이 위치·체력·전투·시야 필수 검증을 모두 통과함을 확인했다. |
| 4 | replay | 🔵 DONE | — | #7 | 샘플 AI 입력을 minified 35,185자로 축소하고 첫 실행 메타데이터 생성 순서를 검증했다. |
| 3 | replay | 🔵 DONE | — | #9, #10 | 영향도와 골드 우위 역전 계산 단위 테스트를 추가했다. |
| 2 | replay | 🔵 DONE | — | — | 기본 분석 뒤 작업 폴더를 삭제하고 `--keep-artifacts` 또는 `keepArtifacts=true`일 때만 보존하도록 했다. |
| 1 | replay | 🔵 DONE | — | — | Fastify 응답 생명주기를 우회하는 SSE 응답에도 허용된 CORS Origin 헤더를 추가했다. |

## 검증 기록

- 2026-08-30 05:39 — Task 61·62 운영 배포 전 기준값을 읽기 전용으로 측정했다. `masteries`는 추정 35,330,564행(데이터 약 10.0GiB, 인덱스 약 6.0GiB), `match_participants`는 추정 11,276,579행이었다. 기존 Champion Detail 첫 집계는 확인 종료 시점에도 546초째 실행 중이었고, 측정 구간의 DB 전체 임시 테이블은 6개(디스크 3개), 행 잠금 대기는 12회·약 87.7초 증가했다. 동시 DataExplorer 쓰기가 있어 잠금 증가분 전체를 통계 작업에 귀속하지 않는다. 마지막 성공 스냅샷은 Champion Detail `2026-07-30 17:54:40`, Mastery `2026-08-12 11:18:15`였으며 운영 서비스·DB·파일은 변경하지 않았다.

- 2026-08-30 04:55 — Task 62에서 최근 패치 선필터와 단일 champion_detail_statistics_source staging 테이블, CTE 기반 메타·카운터 집계, 단계별 실행 시간 로그, 마이그레이션·운영 검증 SQL을 추가했다. 구조 단위 테스트, 백엔드 전체 테스트·빌드와 격리 MySQL 8.0 통합 테스트를 통과했으며 운영 규모의 시간·임시 공간·잠금 비교 전까지 WIP로 유지한다.

- 2026-08-30 04:33 — Task 61에서 숙련도 통계를 챔피언별 dirty queue 기반 증분 집계로 전환하고, 온라인 커버링 인덱스·변경 트리거·동시 갱신 보존 cutoff·materialized 조회·운영 검증 SQL을 추가했다. 관련 패키지 테스트를 통과했으며 운영 규모 DB의 정확도·실행 시간·잠금·쿼리 계획 검증 전까지 WIP로 유지한다.

- 2026-08-21 — Task 69에서 `DATA_EXPLORER_ALERT_DATABASE_BYTES`에 기존 정수 바이트와 1024 기반 `K/KB/M/MB/G/GB/T/TB` 및 소수 단위를 지원하고, 유효값·잘못된 값·범위 초과 테스트와 전체 `go test ./...`, `go build ./...`, `git diff --check`를 통과했다.
- 2026-08-21 — Task 67에서 DataExplorer 일일 사용량·큐·추정 테이블 성장·DB/임시 공간 메트릭과 중복 억제 임계치 알림을 구현했다. `20260821_001` 일별 기준선 마이그레이션과 운영 예산 문서를 추가했으며 `go test ./...`, `go build ./...`, `git diff --check`를 통과했다. 실제 DB에는 실행하지 않았다.
- 2026-08-21 — Task 60에서 9개 기존 SQL 이력을 체크섬·dirty 상태와 함께 `schema_migrations`에 기록하는 순차 실행기를 도입했다. 일반 시작은 기본 `validate`, 명시적 `migrate` 명령만 `up`으로 동작하며, `summoner_matches` 복합 PK 백필은 미러 트리거와 DB cursor로 재시작 가능하다. `go test ./...`, `go build ./...`를 통과했고 운영 DB에는 실행하지 않았다.
- 2026-08-20 — Task 59에서 DataExplorer 처리 상태와 재탐색 cooldown을 job과 분리하고 레거시 상태 백필 및 완료 job/source의 재시작 가능한 소량 정리를 구현했다. 삭제는 기본 비활성으로 두었고 `go test ./...`, `go build ./...`, `git diff --check`를 통과했다.
- 2026-08-20 — Task 58에서 DataExplorer 무제한 관계 확장을 제거하고 안전한 opt-in 기본값, 깊이·예산 경계 및 재시작 동작을 구현했으며 `go test ./...`와 `go build ./...`를 통과했다.
- 2026-08-20 — `rofl-parser@26.16.1` 공식 타입·ESM/CJS exports와 bundled 16.16 artifact를 실제 리플레이로 검증하고 Task 41을 완료했다.
- 2026-08-20 — `docs/reports/001-prod-teamgg-schema.md`의 운영 DB 용량·수집 구조 분석을 중복 제거해 backend 후속 작업 58~67로 분리했다.
- 2026-08-20 — 백엔드·프론트엔드·리플레이 작업을 루트 표 하나로 통합했다.
- 기존 리플레이 문서의 TODO 10개는 최신 루트 보드의 같은 항목과 중복되어 한 번만 유지했다.
- 여러 프로젝트가 포함된 완료 항목은 단일 태그 규칙을 지키기 위해 앱별 행으로 분리했다.
- 기존 문서에 완료 날짜가 없던 이력은 추정하지 않고 `—`로 유지했다.
