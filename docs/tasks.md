# team.gg 작업 관리

이 문서는 team.gg 모노레포 전체 작업의 단일 관리 지점입니다.

## 관리 규칙

- 모든 새 작업은 `🔴 TODO`로 등록합니다.
- 실제 구현을 시작하기 직전에 `🟡 WIP`로 변경합니다.
- 구현과 관련 검증을 모두 마친 작업만 `🟢 DONE`으로 변경합니다.
- 솔로 개발 흐름을 위해 실제 구현 묶음은 한 번에 하나만 `WIP`로 둡니다.
- 항목은 최신이자 가장 큰 Index가 위에 오도록 정렬합니다.
- `Category`는 `backend`, `frontend`, `replay`, `unclassified` 중 하나만 사용합니다.
- 완료되지 않은 작업의 `Completed (KST)`는 `—`로 기록합니다.
- 작업 문구와 진행·검증 기록은 한국어로 작성합니다.
- 앱별 기존 작업 문서는 과거 기록 보관용이며, 신규 작업과 현재 진행 상태는 이 문서에서만 관리합니다.

## 작업 보드

| Index | Category | Status | Completed (KST) | Item | Completion criteria |
|---:|---|---|---|---|---|
| 16 | replay | 🟢 DONE | 2026-08-08 22:21:00 | 리플레이 서버 구조화 로그 강화 | Pino 기반 로그에 ISO 시간과 서비스 문맥을 추가하고 직접·스트림·공유 분석의 시작, 제한된 진행률, 완료 시간, 실패 정보를 일관되게 기록한다. |
| 15 | backend | 🟢 DONE | 2026-08-08 21:56:05 | Docker Data Dragon 저장 경로 영속화 | Docker에서 백엔드 프로젝트 루트를 `/app`으로 고정해 Data Dragon과 통계 캐시가 `backend_datafiles` 볼륨에 저장되도록 하고 회귀 테스트를 통과한다. |
| 14 | backend | 🟢 DONE | 2026-08-08 21:32:06 | Docker 환경 로딩 로그 개선 및 원격 Redis 접속 오류 진단 | Compose 프로세스 환경 주입을 실패로 오해하지 않도록 로그를 수정하고, 원격 Docker에서 호스트 Redis 접속이 거부되는 원인을 확인한다. |
| 13 | replay | 🟢 DONE | 2026-08-08 16:12:10 | 리플레이 서버 잔여 TODO 통합 이관 | 리플레이 서버 문서의 모든 미완료 작업이 루트 보드에 존재하는지 대조하고 앱 문서에서는 미완료 섹션을 제거한다. |
| 12 | unclassified | 🟢 DONE | 2026-08-08 16:09:35 | 작업 보드 Category 컬럼 도입 | `Updated`를 제거하고 `Index` 다음에 제한된 카테고리 값을 갖는 `Category` 컬럼을 추가한다. |
| 11 | unclassified | 🟢 DONE | 2026-08-08 16:04:59 | 모노레포 통합 작업 보드 생성 | 루트 보드를 만들고 기존 미완료 작업을 이관하며 기존 앱 문서에 보관 안내를 표시한다. |
| 10 | unclassified | 🔴 TODO | — | 개발 Docker 프로필과 세 서비스 smoke test | Docker Desktop 환경에서 `docker compose --profile development build` 후 프론트엔드, 백엔드, 리플레이 분석 서버의 기동과 기본 요청을 확인한다. |
| 9 | replay | 🔴 TODO | — | 최우선: 배포 환경 공유 분석 수동 통합 검증 | 방장 생성·업로드·삭제, 참여자 진행률·결과 조회, 권한 거부, 중복·만료·재사용 티켓, 중단 후 복구를 확인한다. 실제 OpenAI 호출 전 모델·호스트·데이터·예상 비용을 설명하고 승인을 받는다. |
| 8 | replay | 🔴 TODO | — | 실제 팀 골드 곡선과 우위 역전 점수 활성화 | `rofl-parser`가 시간대별 개인·팀 골드와 경험치를 제공하면 실제 사건 영향도 계산에 연결하고 검증한다. |
| 7 | replay | 🔴 TODO | — | 중립 오브젝트 스틸 근거 추가 | 사건별 팀 정보와 강타 근거를 사용해 스틸 여부를 확정하고 fixture로 검증한다. |
| 6 | replay | 🔴 TODO | — | 실제 와드와 시야 이벤트 연결 | `detectorWardNetId`가 디코딩되면 시야 진입·이탈을 해당 와드와 연결하고 검증한다. |
| 5 | replay | 🔴 TODO | — | 정밀 합류 시간과 4대5 상태 계산 | 10초 이동 샘플보다 정밀한 근거로 합류 시각과 인원 불균형 상태를 계산하고 검증한다. |
| 4 | replay | 🔴 TODO | — | 대안 행동·개인 피드백 AI 응답 스키마 | 최종 출력 스키마를 구현하고 구조 검증 테스트를 통과한다. |
| 3 | replay | 🔴 TODO | — | 사건 dossier를 실제 OpenAI 분석에 연결 | 사용자 승인 후 `06-event-dossiers.json`을 입력에 연결하고 결과를 검증한다. |
| 2 | replay | 🔴 TODO | — | 보존 디코딩 결과 기반 통합 테스트 | 보존된 실제 결과를 fixture로 구성하고 정제부터 AI 입력 직전까지 회귀 테스트를 통과한다. |
| 1 | replay | 🔴 TODO | — | decoder artifact 모듈 경계 해결 | `rofl-parser`에서 ESM/CommonJS 경계를 해결하고 분석기 우회 코드 없이 빌드·실행한다. |

## 검증 기록

- 2026-08-08 22:21:00 KST — Index 16
  - Fastify 내장 Pino에 ISO UTC 시간, 서비스명, 버전, 환경 및 민감 헤더 마스킹을 추가했다.
  - 직접·스트림·공유 분석에서 시작, 단계 변경 또는 5% 단위 진행, 완료 시간, 실패를 구조화해 기록한다.
  - 서버 시작 시 비밀값을 제외한 실행 설정 요약을 기록한다.
  - `npm run typecheck`, 13개 테스트, `npm run build`, `docker compose build replay-analyzer`를 통과했다.
- 2026-08-08 21:56:05 KST — Index 15
  - Docker 이미지에 `APP_PROJECT_ROOT=/app`, 작업 디렉터리 `/app`, 볼륨 `/app/datafiles`가 함께 설정됨을 확인했다.
  - `GetProjectRootDirectory`의 환경 기반 경로 회귀 테스트와 `go test ./...`를 통과했다.
  - `docker compose build backend`를 통과했다.
  - named volume의 원격 호스트 실제 경로 확인 방법을 `DOCKER.md`에 추가했다.
- 2026-08-08 21:32:06 KST — Index 14
  - Compose가 `apps/backend/.env.docker`를 선택하고 필수 환경변수 키를 프로세스에 주입함을 값 노출 없이 확인했다.
  - 로컬 `.env`가 없는 Docker 실행이 정상 경로임을 명시하도록 시작 로그를 수정했다.
  - 원격 오류의 `172.17.0.1:6379`가 Docker 브리지에서 호스트 Redis로 접속하는 경로이며, 호스트 루프백 전용 Redis에서는 연결이 거부됨을 확인했다.
  - 저장소 내부 임시 `GOCACHE`를 사용한 `go test ./...`를 통과했다.
- 2026-08-08 16:12:10 KST — Index 13
  - 리플레이 서버의 Markdown 문서 전체에서 미완료 체크박스와 `TODO/WIP` 섹션을 검색했다.
  - 발견된 미완료 작업 10개가 루트 보드의 Index 1~10에 모두 반영되어 있음을 대조했다.
  - 앱별 문서에서 `WIP/TODO` 섹션을 제거하고 완료 이력만 보존했다.
- 2026-08-08 16:09:35 KST — Index 12
  - 표 헤더에서 `Updated (KST)`를 제거하고 `Index` 다음에 `Category`를 추가했다.
  - 기존 항목을 `replay` 또는 `unclassified`로 분류했다.
  - 허용 카테고리 규칙을 문서에 명시했다.
- 2026-08-08 16:04:59 KST — Index 11
  - 기존 `apps/lol-replay-analyzer/docs/tasks.md`의 미완료 항목 10개를 루트 보드에 이관했다.
  - 기존 앱 문서에 루트 보드 링크와 이관 상태를 표시했다.
  - `git diff --check`를 통과했다.
