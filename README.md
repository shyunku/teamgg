# team.gg

League of Legends 전적, 내전 팀 구성, 통계 및 ROFL 리플레이 분석 서비스를 함께 관리하는 모노레포입니다.

## 구조

```text
teamgg-frontend/             Svelte 웹 클라이언트
teamgg-backend/              Go API 서버
teamgg-lol-replay-analyzer/  Node.js/TypeScript ROFL 분석 서버
```

## 개발 실행

각 서비스의 환경 파일을 준비한 뒤 세 서비스를 함께 실행합니다.

```powershell
Copy-Item .env.compose.example .env.compose
Copy-Item teamgg-backend/.env.docker.example teamgg-backend/.env.docker
Copy-Item teamgg-lol-replay-analyzer/.env.docker.example teamgg-lol-replay-analyzer/.env.docker
docker compose --env-file .env.compose --profile development up --build
```

상세한 Docker 및 운영 배포 방법은 [DOCKER.md](./DOCKER.md)를 참고하세요.

## 운영 실행

프론트엔드를 S3에서 제공하는 운영 환경에서는 서버 두 개만 실행합니다.

```powershell
docker compose --env-file .env.compose up -d --build backend replay-analyzer
```
