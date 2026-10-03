# team.gg 프로젝트 명세

## 목적

team.gg는 League of Legends 전적·통계 조회, 내전 팀 구성 및 밸런싱, ROFL 리플레이 AI 분석을 하나의 서비스로 제공하는 모노레포입니다.

## 애플리케이션

| 경로 | 역할 | 주요 진입점 |
|---|---|---|
| `apps/frontend` | Svelte 기반 웹·Electron 클라이언트 | `src/App.svelte`, `src/routers/MainRouter.js` |
| `apps/backend` | Go/Gin 기반 team.gg API, 데이터 수집 및 통계 | `main.go`, `controllers/router.go` |
| `apps/lol-replay-analyzer` | Node.js/TypeScript 기반 ROFL 디코딩 및 AI 분석 | `src/server.ts` |
| `apps/admin` | Go 기반 관리자 게이트웨이 및 서비스 상태 수집 | `main.go` |

## 문서 구성

| 경로 | 내용 |
|---|---|
| `docs/project.md` | 확정된 요구사항과 구조 (이 문서) |
| `docs/plans.md` | 진행 중인 스프린트와 미확정 선택지 |
| `docs/tasks.md`, `docs/tasks_done.md` | 미완료·완료 작업 보드 |
| `docs/tasks/{3자리 Index}.md` | 작업별 상세 기록 |
| `docs/cautions.md` | 운영·배포 시 반복해서 주의할 사항 |
| `docs/operations/` | 백엔드 유지보수 명령과 운영 절차 |
| `docs/replay/` | 리플레이 분석 입력 스키마와 연구 기록 |
| `docs/plans/` | 다단계 설계 계획 |
| `docs/reports/` | 운영 검증·장애·조사 보고서 |

## 관리자 시스템

- 관리자 API는 일반 백엔드와 별도 프로세스로 배포한다.
- 관리자 화면은 기존 team.gg 프론트엔드의 `/admin` 경로에 통합한다.
- 화면 코드의 공개 여부를 권한 경계로 사용하지 않으며 모든 접근 권한은 서버에서 검증한다.
- 관리자 서버에는 team.gg JWT 서명키와 Docker 소켓을 제공하지 않는다.
- 관리자 서버는 브라우저에서 받은 team.gg access token을 내부 인증 API에 전달하고, 백엔드가 사용자 역할을 검증한다.
- 백엔드 내부 관리자 API는 별도 공유 비밀키로 보호한다.
- 관리자 역할은 DB의 `user_roles`가 기준이며, 초기 운영자는 환경변수 allowlist로 부트스트랩할 수 있다.
- 조회 행위는 `admin_audit_logs`에 기록하며, 응답과 감사 메타데이터에서 token·cookie·password·secret 등의 민감 필드를 제거한다.
- 로그 화면은 Docker 소켓이나 임의 파일 접근 대신 관리 감사 로그와 백엔드가 명시적으로 저장한 운영 이벤트만 제공한다.

## 배포 원칙

- 운영 Compose는 `backend`, `replay-analyzer`, `admin`을 실행한다.
- 프론트엔드는 기존 방식대로 정적 빌드 후 S3/CloudFront에 배포한다.
- 관리자 서버는 team.gg 및 localhost Origin만 허용하고, 내부 백엔드는 Compose 서비스 주소로 접근한다.
- 일반 서버 시작은 스키마를 검증만 하며, 마이그레이션·백필·데이터 정리는 명시적 유지보수 명령(`migrate`, `backfill-numeric-keys`, `cleanup-retention`)으로만 실행한다.

## 데이터 수집 (DataExplorer)

- DataExplorer는 명시적 opt-in이며 탐색 깊이와 신규 소환사·경기 일일 예산으로 범위를 제한한다.
- 소환사·경기별 처리 상태와 재탐색 cooldown을 작업 큐와 분리해 관리한다.
- 큐·예산·DB 용량·순증가량을 저빈도 메트릭 로그로 남기고 임계치 최초 발생·정상화 시 알린다.

## 데이터 보존

- 원본 경기 상세는 최신 8개 패치만 보존하고, 이전 패치 경기와 하위 행을 자식 → 부모 순서의 제한 배치로 삭제한다. 외부 아카이브는 두지 않는다.
- 숫자 경기·참가자 identity 매핑은 재수집 시 같은 ID를 쓰도록 삭제하지 않는다.
- 운영 호스트의 systemd timer가 7일마다 KST 02:00~04:00 창에서 `cleanup-retention`을 실행한다.
- 기본 실행은 백엔드를 멈추지 않는 online 모드이며, READ COMMITTED 짧은 배치와 락 충돌 재시도, DB 부하·디스크 하한 가드, 최대 2시간 제한을 적용한다.
- 실패나 work limit 미완료는 운영자 Discord 채널 웹훅으로 알린다.

## 숙련도 저장과 통계

- 숙련도는 숫자 소환사 키 기반 `masteries_numeric_v2`에만 저장하며, 문자열 PUUID 기반 legacy `masteries`는 제거됐다.
- 숙련도 통계는 전체 테이블을 주기적으로 그룹화하지 않고 챔피언별 materialized aggregate를 조회한다.
- 변경 트리거는 영향을 받은 챔피언만 dirty queue에 기록하며, 중복 변경은 챔피언당 한 행으로 합쳐진다.
- 수집기는 챔피언별 커버링 인덱스 범위 스캔으로 집계를 갱신하고 DB cutoff 이후 발생한 변경을 다음 실행을 위해 보존한다.

## Champion Detail·메타 통계

- 최근 패치 경기만 `matches_game_version_index`로 먼저 제한한다.
- 참가자·룬·아이템·동일 라인 상대 정보를 경기 10개 단위 증분 source로 정규화하고, 버전별 cursor와 processed-match 마커로 재시작과 늦게 유입된 경기를 처리한다.
- 전역 통계 advisory lock으로 여러 서버 인스턴스가 동시에 집계하지 않는다.
- 메타와 카운터 통계는 source 데이터에 대한 CTE 집계로 계산하며 외부 API 응답 스키마는 유지한다.

## 숫자 키 기반 스키마 v2

- Riot PUUID·match ID·참가자 UUID는 외부 식별자로 유지하고, DB 내부 관계에는 `BIGINT UNSIGNED` 숫자 키를 사용한다.
- `summoners.summoner_pk`, `matches.match_pk`, `match_participants.match_participant_pk`를 새 내부 키로 사용한다.
- 기존 `summoners.id`와 `match_participants.participant_id`는 의미가 다른 레거시 필드이므로 숫자 PK로 재사용하지 않는다.
- 신규 쓰기는 트리거와 애플리케이션 identity 선점으로 숫자 키를 즉시 받고, 기존 행은 `backfill-numeric-keys` 유지보수 명령의 재시작 가능한 keyset 배치로 채운다.
- 백필 트랜잭션은 애플리케이션 쓰기와 같은 READ COMMITTED로 실행해 백엔드 무중단 실행 시 교착을 피한다.
- 전환은 additive foundation, 신규 쓰기 동기화, 제한된 keyset 백필, 동일성 검증, 하위 FK 이관, 읽기 전환, 제약조건 전환, 레거시 정리 순서로 수행한다.
- 룬 테이블은 하위 FK 백필 대신 룬 평탄화 단계에서 숫자 참가자 키 기반 새 구조로 옮긴다.
- 전체 백필과 운영 검증 전에는 문자열 키 또는 기존 PK/FK를 제거하지 않는다.
- 세부 단계와 완료 조건은 `docs/plans/numeric-key-schema-v2.md`를 따른다.
