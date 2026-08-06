# teamgg-lol-replay-analyzer

team.gg용 League of Legends ROFL 분석 서버입니다. 클라이언트가 업로드한 `.rofl` 파일에서 경기 종료 통계와 제한된 주요 이벤트를 추출한 뒤, OpenAI Responses API를 이용해 팀별 승패 요인과 개인별 피드백을 생성합니다.

## 요구 사항

- Node.js 20 이상
- OpenAI API 키와 사용할 모델 이름
- ROFL과 정확히 일치하는 `League of Legends.exe`
  - `LOL_EXECUTABLE_PATH`를 지정하지 않으면 `lol-exe-downloader`가 해당 빌드를 자동으로 받습니다.
  - 처음 보는 실행 파일에서 decoder artifact를 만들 때 Python 3와 Python `unicorn` 패키지가 필요할 수 있습니다.

## 실행

```powershell
Copy-Item .env.example .env
npm install
npm run dev
```

`.env`에서 최소한 다음 두 값을 설정해야 실제 분석이 가능합니다.

```dotenv
OPENAI_API_KEY=...
OPENAI_MODEL=사용할_Responses_API_모델_ID
```

키가 없어도 서버와 `/health`는 기동되지만 분석 API는 `503 AI_NOT_CONFIGURED`를 반환합니다.

`npm start`는 `npm run build`로 생성된 `dist/src/server.js`를 실행합니다. Docker에서는 공개 decoder registry를 소비하므로 Python/Unicorn builder를 이미지에 포함하지 않습니다. 새 패치의 decoder artifact는 애플리케이션 서버가 아니라 별도 builder 환경에서 생성·배포해야 합니다.

## 분석 산출물과 연구 모드

기본 동작은 분석 완료 뒤 `.work` 작업 폴더를 삭제합니다. 로컬 연구나 디버깅에서는 다음처럼 명시적으로 보존합니다.

```powershell
npm run analyze:rofl -- ".\KR-8326522247.rofl" --keep-artifacts
```

보존된 작업 폴더의 `<decoded>/refined/`에는 작고 검증된 자료만 남습니다.

- `metadata.json`: 원본의 대용량 raw/base64 필드를 제거한 경기·플레이어 종료 통계
- `packets/deaths.json`: `paramHint` 하위 16비트로 `p1`~`p10`에 매핑한 사망 이벤트
- `packets/level-ups.json`: 검증된 레벨업 이벤트
- `packets/manifest.json`: 포함·제외한 패킷 및 신뢰도 근거

`<decoded>/process/prompt-assets/`에는 AI에게 실제 전달한 증거 JSON이, `process/result.md`에는 최종 AI 응답이 저장됩니다. HTTP API에서는 `?keepArtifacts=true`를 붙이면 같은 방식으로 보존됩니다.
## API

### 상태 확인

`GET /health`

### 한 번에 분석

```http
POST /v1/replays/analyze?language=Korean&focusPlayer=닉네임%23태그
Content-Type: multipart/form-data
```

파일 필드 이름은 `replay`입니다.

```bash
curl -F "replay=@KR-1234567890.rofl" "http://localhost:7720/v1/replays/analyze?language=Korean"
```

응답에는 `requestId`, AI에 전달된 정규화 `digest`, Markdown `analysis`, `model`이 들어갑니다.

### 스트리밍 분석

```http
POST /v1/replays/analyze/stream?language=Korean
Content-Type: multipart/form-data
Accept: text/event-stream
```

POST 업로드이므로 브라우저 `EventSource`가 아니라 `fetch()` 응답 스트림 또는 `XMLHttpRequest`의 점진적 응답을 사용해야 합니다. 업로드율과 응답 스트림을 동시에 표시해야 하는 클라이언트에는 `XMLHttpRequest`가 적합합니다.

```bash
curl -N -F "replay=@KR-1234567890.rofl" "http://localhost:7720/v1/replays/analyze/stream?language=Korean"
```

SSE 이벤트는 `started`, `progress`, `analysis_delta`, `result`, `error` 순으로 사용합니다. `progress`의 `progress` 값은 서버 분석 전체를 기준으로 한 `0`~`1` 값이며, `stage`와 `message`에는 현재 단계와 사용자용 설명이 들어갑니다. `result`에는 최종 전체 결과가 들어갑니다.

team.gg 공유 내전 분석에서는 백엔드가 발급한 단기 서명 티켓을 `X-Replay-Upload-Ticket` 헤더에 넣어 `POST /v1/replays/jobs/:jobId/upload/stream`으로 직접 업로드합니다. 이전 클라이언트 호환을 위해 `ticket` 쿼리도 허용하지만 새 클라이언트는 URL과 접근 로그에 티켓을 남기지 않습니다. 분석 서버는 `TEAMGG_API_BASE_URL`의 내부 콜백으로 단계별 진행률과 최종 Markdown을 저장합니다. `REPLAY_ANALYZER_SHARED_SECRET`은 team.gg 백엔드와 동일한 긴 임의 문자열이어야 하며, 진행 콜백은 최대 초당 한 번 전송됩니다.

## 서버 없이 ROFL 파일 직접 테스트

기본 명령은 디코딩 진행 상황을 stderr에 표시하면서 AI 분석 본문을 실시간으로 stdout에 출력합니다.

```powershell
npm run analyze:rofl -- "C:\replays\KR-1234567890.rofl"
```

완료 후 한 번에 출력하려면 `--direct`를 사용합니다.

```powershell
npm run analyze:rofl -- "C:\replays\KR-1234567890.rofl" --direct
```

추가 예시:

```powershell
# 특정 플레이어를 중점 분석하고 Markdown 파일로 저장
npm run analyze:rofl -- "C:\replays\KR-1234567890.rofl" `
  --focus-player "닉네임#KR1" `
  --output ".\analysis.md"

# digest를 포함한 전체 JSON 확인
npm run analyze:rofl -- "C:\replays\KR-1234567890.rofl" --json
```

모든 옵션은 `npm run analyze:rofl -- --help`에서 확인할 수 있습니다. CLI도 `.env`의 실행 파일, 캐시, timeout, OpenAI 설정을 그대로 사용합니다.

## 환경 변수

| 변수 | 기본값 | 설명 |
|---|---:|---|
| `HOST` | `0.0.0.0` | listen 주소 |
| `PORT` | `7720` | listen 포트 |
| `MAX_UPLOAD_MB` | `250` | 파일별 업로드 제한 |
| `MAX_CONCURRENT_ANALYSES` | `1` | 동시에 실행할 디코딩/AI 작업 수 |
| `ANALYSIS_TIMEOUT` | `20m` | 대기열 포함 제한 시간 (`ms`, `s`, `m`, `h`) |
| `WORK_DIR` | `./.work` | 요청별 임시 파일 경로 |
| `ROFL_CACHE_DIR` | `./.rofl-cache` | 실행 파일 및 decoder artifact 캐시 |
| `KEEP_ARTIFACTS` | `false` | 디버깅을 위해 요청 작업물을 보존할지 여부 |
| `LOL_EXECUTABLE_PATH` | 비어 있음 | 지정 시 자동 다운로드 대신 이 실행 파일 사용 |
| `LOL_DOWNLOAD_REGION` | `KR` | 자동 다운로드 region |
| `OPENAI_API_KEY` | 필수 | 서버 전용 API 키 |
| `OPENAI_MODEL` | 필수 | 사용할 OpenAI 모델 ID |
| `OPENAI_BASE_URL` | 비어 있음 | 호환 API/프록시가 필요할 때만 지정 |
| `OPENAI_REASONING_EFFORT` | 비어 있음 | `none`, `low`, `medium`, `high`, `xhigh`, `max`; 비워두면 모델 기본값 사용 |
| `OPENAI_MAX_OUTPUT_TOKENS` | `6000` | 분석 최대 출력 토큰 |
| `CORS_ORIGINS` | `http://localhost:8080` | 쉼표로 구분한 허용 origin |

## 동작 및 제한

1. multipart 업로드는 메모리가 아니라 요청별 임시 파일로 스트리밍됩니다.
2. ROFL footer의 종료 통계를 직접 읽어 거대한 Base64 원본이 포함된 parser `metadata.json`을 다시 메모리에 올리지 않습니다.
3. `rofl-parser`는 compact 형식으로 디코딩하며, 주요 이벤트 관련 packet만 최대 500건까지 AI 입력에 포함합니다.
4. 분석 완료/실패 시 요청 작업물은 삭제되고 실행 파일과 decoder artifact 캐시는 재사용됩니다.
5. parser의 구조 해석 성공은 게임 의미의 완전한 검증을 뜻하지 않습니다. 종료 통계를 우선하고 불확실한 사건을 지어내지 않도록 프롬프트를 제한했습니다.

실제 ROFL 호환성은 리플레이 버전, 정확한 실행 파일 확보 여부, 시험 버전 `rofl-parser` decoder 품질에 영향을 받습니다. 문제 재현 시 `KEEP_ARTIFACTS=true`, `LOG_LEVEL=debug`로 설정하면 중간 산출물과 parser stage 로그를 확인할 수 있습니다.
