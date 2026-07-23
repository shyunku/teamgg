# 🎮 team.gg Server

League of Legends 내전(커스텀 게임) 매칭, 참가자 밸런스 조정 및 전적/통계 분석 서비스를 제공하는 **team.gg**의 백엔드 API 서버입니다. Go 언어와 Gin 프레임워크를 기반으로 구축되었으며, 실시간 통신 및 백그라운드 데이터 수집 루프를 포함하고 있습니다.

---

## 🛠️ 기술 스택 (Tech Stack)

- **Language**: Go 1.26.5
- **Web Framework**: [Gin Web Framework](https://github.com/gin-gonic/gin)
- **Realtime**: [go-socket.io](https://github.com/googollee/go-socket.io)
- **Database**: MySQL / PostgreSQL (with [sqlx](https://github.com/jmoiron/sqlx))
- **Cache / In-Memory DB**: Redis ([go-redis/v9](https://github.com/redis/go-redis))
- **Third-Party API**: Riot Games API & Riot Sign On (RSO)

---

## 📁 디렉토리 구조 (Directory Structure)

```text
team.gg-server/
├── controllers/          # API 핸들러 및 라우터 정의
│   ├── middlewares/      # Gin 미들웨어 (인증, 로깅 등)
│   ├── socket/           # 웹소켓(socket.io) 통신 핸들러
│   └── v1/               # 버전 1 API 라우터 (인증, 내전 관리, 통계 등)
├── models/               # 데이터베이스 모델 및 DTO 정의 (소환사, 매치, 참가자 등)
├── service/              # 핵심 비즈니스 로직 및 백그라운드 스케줄러
│   ├── statistics/       # 챔피언, 티어, 숙련도 통계 주기적 집계 로직
│   └── explorer.go       # 백그라운드 데이터 수집 및 Riot API 주기적 연동
├── libs/                 # 공통 라이브러리 (DB 커넥션, 암호화 패키지)
├── third_party/          # 외부 라이브러리 및 Riot API 클라이언트
├── scripts/              # 빌드, 데몬 실행, 정지 등 셸 스크립트
├── go.mod / go.sum       # 의존성 관리 파일
└── main.go               # 서버 시작점 (Initialization & Runner)
```

---

## ⚙️ 환경 설정 (Configuration)

프로젝트 루트 디렉토리에 `.env` 파일을 생성하고 아래의 환경 변수들을 설정해야 합니다.

```env
# Server Port
APP_SERVER_PORT=8080

# Database Configuration (MySQL / PostgreSQL)
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=team_gg

# JWT Secret Keys
JWT_ACCESS_SECRET=your_jwt_access_secret
JWT_ACCESS_EXPIRE=3600
JWT_REFRESH_SECRET=your_jwt_refresh_secret
JWT_REFRESH_EXPIRE=604800

# Riot Sign On (RSO) & Riot API Client
RSO_CLIENT_ID=your_rso_client_id
RSO_CLIENT_SECRET=your_rso_client_secret
RSO_CLIENT_CALLBACK_URI=http://localhost:8080/v1/auth/callback

# Debug Mode (true/false)
DEBUG=true
IS_PROD=false

# Background data explorer (optional)
DATA_EXPLORER_ENABLED=true
DATA_EXPLORER_SUMMONER_WORKERS=1
DATA_EXPLORER_MATCH_WORKERS=2
DATA_EXPLORER_DAILY_SUMMONER_BUDGET=500
DATA_EXPLORER_DAILY_MATCH_BUDGET=1500
DATA_EXPLORER_MATCH_COUNT=3
DATA_EXPLORER_BOOTSTRAP_BATCH_SIZE=500
DATA_EXPLORER_BOOTSTRAP_INTERVAL=5s
DATA_EXPLORER_POLL_INTERVAL=1s
DATA_EXPLORER_LEASE=5m
DATA_EXPLORER_MAX_ATTEMPTS=8
DATA_EXPLORER_DEBUG=false
DATA_EXPLORER_STATUS_LOG_INTERVAL=30s

# Statistics snapshots (optional; each loop is disabled unless explicitly enabled)
STATISTICS_TIER_ENABLED=true
STATISTICS_TIER_PERIOD=12h
STATISTICS_TIER_INITIAL_DELAY=30s

STATISTICS_MASTERY_ENABLED=true
STATISTICS_MASTERY_PERIOD=12h
STATISTICS_MASTERY_INITIAL_DELAY=2m

STATISTICS_CHAMPION_DETAIL_ENABLED=true
STATISTICS_CHAMPION_DETAIL_PERIOD=24h
STATISTICS_CHAMPION_DETAIL_INITIAL_DELAY=10m

STATISTICS_RETRY_DELAY=5m
STATISTICS_LOCK_RETRY_DELAY=15s
STATISTICS_LOCK_TIMEOUT=1s
```

`DATA_EXPLORER_*_BUDGET`에 `0`을 지정하면 해당 일일 제한을 사용하지 않습니다. 워커 수와 예산은 Riot API 제품 키의 실제 제한에 맞춰 조정해야 합니다.

통계 시간 값은 Go duration 형식(`30s`, `5m`, `12h`)을 사용합니다. 각 통계는 서로 다른 초기 지연으로 시작해 서버 부팅 시 DB 집계가 한꺼번에 실행되지 않습니다. `STATISTICS_LOCK_RETRY_DELAY`는 다른 통계가 실행 중일 때의 짧은 재시도 간격이고, `STATISTICS_RETRY_DELAY`는 실제 집계 오류가 발생했을 때의 재시도 간격입니다. 여러 서버 인스턴스가 실행되어도 MySQL advisory lock과 `statistics_snapshots` 공용 캐시를 이용해 동일 통계를 중복 계산하지 않습니다.

---

## 🚀 실행 및 빌드 방법 (How to Run)

### 1. 사전 요구사항 (Prerequisites)
- [Go](https://go.dev/doc/install) 1.26.5 설치
- MySQL 또는 PostgreSQL 데이터베이스 인스턴스 실행
- Redis 서버 실행

### 2. 의존성 패키지 설치
```bash
go mod download
```

### 3. 데이터베이스 스키마 적용
프로젝트 루트의 `scheme.ddl` 스키마 파일을 관계형 데이터베이스에 적용합니다.

기존 설치에서 DataExplorer 큐로 전환할 때는 서버 쓰기를 중단한 뒤 `migrations/20260720_add_data_explorer_queue.sql`을 적용합니다. 이 마이그레이션은 키가 없던 `summoner_matches`를 7일 단위로 중복 제거하여 교체하며, 검증을 위해 기존 테이블을 `summoner_matches_legacy_20260720` 이름으로 남깁니다.

통계 공용 캐시 테이블은 서버 시작 시 자동 생성됩니다. 대규모 기존 DB에서는 통계 조회용 인덱스를 추가하는 `migrations/20260724_add_statistics_indexes.sql`을 트래픽이 적은 유지보수 시간에 한 번 적용하는 것을 권장합니다.

### 4. 개발 서버 실행
```bash
go run main.go
```

### 5. 빌드 및 배포 스크립트 (Linux environment)
`scripts/` 디렉토리에 유틸리티 스크립트가 포함되어 있습니다.
- **빌드**: `bash scripts/build.sh`
- **백그라운드 실행(데몬)**: `bash scripts/daemon.sh`
- **서버 정지**: `bash scripts/stop.sh`
- **재빌드 및 재시작**: `bash scripts/build_and_restart.sh`
