# 🎮 team.gg Server

League of Legends 내전(커스텀 게임) 매칭, 참가자 밸런스 조정 및 전적/통계 분석 서비스를 제공하는 **team.gg**의 백엔드 API 서버입니다. Go 언어와 Gin 프레임워크를 기반으로 구축되었으며, 실시간 통신 및 백그라운드 데이터 수집 루프를 포함하고 있습니다.

---

## 🛠️ 기술 스택 (Tech Stack)

- **Language**: Go 1.19+
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
```

---

## 🚀 실행 및 빌드 방법 (How to Run)

### 1. 사전 요구사항 (Prerequisites)
- [Go](https://go.dev/doc/install) 1.19 버전 이상 설치
- MySQL 또는 PostgreSQL 데이터베이스 인스턴스 실행
- Redis 서버 실행

### 2. 의존성 패키지 설치
```bash
go mod download
```

### 3. 데이터베이스 스키마 적용
프로젝트 루트의 `scheme.ddl` 스키마 파일을 관계형 데이터베이스에 적용합니다.

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
