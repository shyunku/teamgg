# team.gg 운영 유의사항

## 운영 DB

- 운영 MySQL 인스턴스는 다른 프로젝트 스키마(`mips`)와 공유한다. buffer pool, binlog 보존 기간 같은 전역 설정 변경은 다른 프로젝트에도 적용된다.
- InnoDB buffer pool은 384MiB(`SET PERSIST`)다. 서버 RAM이 3.8GiB이고 swap을 이미 쓰고 있어 더 늘리기 전에 메모리 여유를 확인한다.
- binlog 보존 기간은 24시간이다. 시점 복구(PITR) 가능 범위도 24시간이다.
- 대량 쓰기 작업은 binlog와 InnoDB 내부 공간을 늘린다. 실행 전 루트 디스크 여유가 12GiB 안전선보다 충분히 높은지 확인한다. 행 `DELETE`로 생긴 공간은 테이블 내부에서 재사용될 뿐 OS 디스크 여유를 바로 늘리지 않는다.
- 숫자 identity 테이블의 식별자 컬럼(`puuid`, `riot_match_id` 등)은 ascii_bin이고 원본 테이블은 utf8mb4라 서로 JOIN하면 조인 순서에 따라 실행 계획이 크게 달라진다. 운영에서 대량 조회·수정 전에 `EXPLAIN FORMAT=TREE`로 해시 조인 또는 인덱스 조인인지 확인한다.
- MySQL 통합 테스트는 격리된 로컬 인스턴스에서만 실행한다. 운영 DB 접속정보(`TEAMGG_NUMERIC_KEY_MYSQL_TEST_FROM_DB_ENV`)로 실행하면 테스트 DB가 운영 인스턴스에 남을 수 있다.

## 유지보수 명령

- `cleanup-retention`, `backfill-numeric-keys` 같은 유지보수 명령은 `docker compose run`으로 새 컨테이너에서 실행되므로 `docker compose build backend` 뒤에는 실행 중인 백엔드를 재시작하지 않아도 새 코드가 적용된다. 컨테이너의 `unhealthy` 표시는 HTTP 헬스체크가 붙은 이미지 때문이며 무시해도 된다.
- 오래된 경기 정리는 7일마다 KST 02:00~04:00에 시작해 최대 2시간 실행된다. 대량 백필이나 DDL은 이 시간과 겹치지 않게 한다.
- `backfill-numeric-keys`는 참가자 단계가 끝나면 같은 실행에서 하위 관계(#71) 단계로 자동 진행한다. 참가자 단계만 돌리려면 `NUMERIC_KEY_BACKFILL_STOP_AFTER_PARTICIPANTS=true`를 준다(해당 코드가 배포된 이미지 기준).
- 대량 작업은 work limit을 두고 나눠 실행하며, 디스크 여유·락 대기·API 응답을 함께 관측한다.

## 원격 작업

- Windows PowerShell에서 여러 줄 스크립트를 SSH 표준입력으로 넘기면 BOM·줄 끝 문자 때문에 명령이 잘못 해석될 수 있다. 스크립트는 LF로 저장해 서버에 복사한 뒤 실행한다.
- 백엔드 중단·복구가 필요한 명령은 원격 단일 셸에서 EXIT trap과 명시적인 timeout을 함께 사용한다.

## 비밀값

- 운영 `.env.retention`의 Discord 웹훅 URL은 그 채널에 글을 쓸 수 있는 비밀값이다. 커밋·로그·채팅에 남기지 않는다.
