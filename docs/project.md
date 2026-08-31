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
## 숙련도 통계 집계

- 숙련도 통계는 `masteries` 전체를 주기적으로 그룹화하지 않고 챔피언별 materialized aggregate를 조회한다.
- `masteries` 변경 트리거는 영향을 받은 챔피언만 dirty queue에 기록하며, 중복 변경은 챔피언당 한 행으로 합쳐진다.
- 수집기는 챔피언별 커버링 인덱스 범위 스캔으로 집계를 갱신하고 DB cutoff 이후 발생한 변경을 다음 실행을 위해 보존한다.
- 최초 마이그레이션은 기존 챔피언을 모두 dirty 상태로 등록해 재시작 가능한 분할 백필을 수행한다.
- 운영 규모 DB의 정확도·쿼리 계획·실행 시간·잠금 검증을 통과한 뒤 Task #61을 완료 처리한다.
## Champion Detail·메타 통계 집계

- 최근 패치 경기 ID를 `matches_game_version_index`로 먼저 제한한다.
- 참가자·룬·아이템·동일 라인 상대 정보를 `champion_detail_statistics_source`의 참가자당 한 행으로 정규화한다.
- 전역 통계 advisory lock 안에서 staging 테이블을 교체하므로 여러 서버 인스턴스가 동시에 덮어쓰지 않는다.
- 메타와 카운터 통계는 staging 데이터에 대한 statement-scoped CTE로 계산하며 외부 API 응답 스키마는 유지한다.
- 운영 규모에서 기존 약 7.8GB 임시 공간 기준과 실행 시간·잠금을 비교한 뒤 Task #62를 완료 처리한다.

## 숫자 키 기반 스키마 v2

- Riot PUUID·match ID·참가자 UUID는 외부 식별자로 유지하고, DB 내부 관계에는 `BIGINT UNSIGNED` 숫자 키를 사용한다.
- `summoners.summoner_pk`, `matches.match_pk`, `match_participants.match_participant_pk`를 새 내부 키로 사용한다.
- 기존 `summoners.id`와 `match_participants.participant_id`는 의미가 다른 레거시 필드이므로 숫자 PK로 재사용하지 않는다.
- 전환은 additive foundation, 신규 쓰기 동기화, 제한된 keyset 백필, 동일성 검증, 하위 FK 이관, 읽기 전환, 제약조건 전환, 레거시 정리 순서로 수행한다.
- 일반 서버 시작은 백필을 실행하지 않으며 `backfill-numeric-keys` 유지보수 명령에서만 명시적으로 실행한다.
- 전체 백필과 운영 검증 전에는 문자열 키 또는 기존 PK/FK를 제거하지 않는다.
- 세부 단계와 완료 조건은 `docs/plans/numeric-key-schema-v2.md`를 따른다.
