# team.gg Docker 실행

이 Compose 구성은 세 저장소만 실행합니다. MySQL과 Redis는 기존 로컬 또는 외부 인프라를 사용합니다.

## 최초 설정

세 저장소의 상위 폴더에서 프로젝트별 환경파일을 만듭니다.

```powershell
Copy-Item .env.compose.example .env.compose
Copy-Item apps/backend/.env.docker.example apps/backend/.env.docker
Copy-Item apps/lol-replay-analyzer/.env.docker.example apps/lol-replay-analyzer/.env.docker
```

각 파일의 역할은 분리되어 있습니다.

- `.env.compose`: 공개 포트와 bind address만 관리하며 애플리케이션 비밀값은 넣지 않음
- `apps/backend/.env.docker`: DB, Redis, Riot, RSO, JWT, 백엔드 실행 설정
- `apps/lol-replay-analyzer/.env.docker`: OpenAI, 업로드, decoder, 분석 서버 실행 설정
- `apps/frontend/.env.dev`, `.env.production`: 기존 프론트 빌드 설정을 그대로 사용

기존 프로젝트 `.env`를 그대로 사용하려면 `.env.compose`에서 경로만 바꿀 수도 있습니다.

```dotenv
BACKEND_ENV_FILE=./apps/backend/.env
ANALYZER_ENV_FILE=./apps/lol-replay-analyzer/.env
```

다음 값은 특히 중요합니다.

- `DB_HOST`: Windows/macOS의 호스트 DB는 `host.docker.internal`; 운영 DB는 실제 접근 가능한 호스트명
- `REDIS_ADDR`: 호스트 Redis는 `host.docker.internal:6379`; 운영에서는 실제 Redis 주소
- `REPLAY_ANALYZER_SHARED_SECRET`: 백엔드와 분석 서버가 함께 쓰는 충분히 긴 임의 문자열
- `REPLAY_ANALYZER_BASE_URL`: 브라우저가 접근할 분석 서버 주소. 개발은 `http://localhost:7720`, 운영은 공개 HTTPS 도메인
- `CORS_ORIGINS`: 개발은 `http://localhost:8080`, 운영은 `https://teamgg.kr,https://www.teamgg.kr`
- `TEAMGG_API_BASE_URL`: Compose가 내부 주소 `http://backend:7713`으로 덮어쓰므로 직접 변경할 필요 없음

PowerShell에서 32바이트 공유 비밀키를 생성하는 예시입니다.

```powershell
[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32)).ToLower()
```

## 개발: 세 서비스 실행

```powershell
docker compose --env-file .env.compose --profile development up --build
```

- 프론트엔드: http://localhost:8080
- 백엔드: http://localhost:7713
- 리플레이 분석 서버: http://localhost:7720

프론트 소스는 bind mount되며 Rollup watch와 livereload가 동작합니다. 백엔드와 분석 서버 소스를 수정한 경우 이미지를 다시 빌드해야 합니다.

```powershell
docker compose --env-file .env.compose up -d --build backend replay-analyzer
```

## 운영: 서버 두 개만 실행

운영 서버에서는 백엔드 `.env.docker`를 다음처럼 설정합니다.

```dotenv
IS_PROD=true
DEBUG=false
REPLAY_ANALYZER_BASE_URL=https://replay-api.teamgg.kr
RSO_CLIENT_CALLBACK_URI=https://api.teamgg.kr/v1/auth/rsoLogin
```

분석 서버 `.env.docker`에는 다음 값을 설정합니다.

```dotenv
NODE_ENV=production
CORS_ORIGINS=https://teamgg.kr,https://www.teamgg.kr
```

루트 `.env.compose`에서는 외부 reverse proxy만 접근하도록 bind address를 제한합니다.

```dotenv
BACKEND_BIND_ADDRESS=127.0.0.1
ANALYZER_BIND_ADDRESS=127.0.0.1
```

프론트엔드는 `development` 프로필에만 있으므로 다음 명령은 백엔드와 분석 서버만 실행합니다.

```bash
docker compose --env-file .env.compose up -d --build backend replay-analyzer
```

운영 reverse proxy에서는 분석 서버 업로드 경로에 최소 `250m` body limit, `ANALYSIS_TIMEOUT`보다 긴 read timeout, SSE buffering 비활성화를 설정해야 합니다. 두 공개 API에는 HTTPS를 사용하고, 컨테이너 포트는 가능하면 위 예시처럼 loopback에만 바인딩합니다.

프론트 운영 빌드는 기존대로 S3에 업로드합니다.

```powershell
Set-Location apps/frontend
npm ci
npm run build
```

업로드 대상은 `public/`이며 운영 번들은 `public/build/production/`에 생성됩니다.

## 관리 명령

```powershell
# 상태
docker compose --env-file .env.compose ps

# 로그
docker compose --env-file .env.compose logs -f backend replay-analyzer

# 재시작
docker compose --env-file .env.compose restart backend replay-analyzer

# 종료
docker compose --env-file .env.compose --profile development down
```

`backend_datafiles`에는 Data Dragon과 통계 캐시가, `replay_decoder_cache`에는 LoL 실행 파일과 decoder artifact가 보존됩니다. `replay_work`에는 처리 중인 대용량 업로드·디코딩 파일이 놓이고 정상 완료/실패 시 애플리케이션이 삭제합니다. `docker compose down -v`는 이 데이터와 캐시 볼륨까지 삭제하므로 필요한 경우에만 사용합니다.
