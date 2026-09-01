# 숫자 키 기반 스키마 v2 전환 계획

## 목적

`summoners.puuid`, `matches.match_id`, `match_participants.match_participant_id` 같은 긴 문자열 식별자가 하위 테이블의 PK·FK·보조 인덱스에 반복되는 구조를 숫자 키 기반으로 전환한다.

외부 API와 Riot 데이터의 식별자는 계속 문자열로 유지하되, DB 내부 관계와 대용량 인덱스에는 `BIGINT UNSIGNED` 숫자 키를 사용한다.

## 숫자 키

| 엔터티 | 기존 식별자 | 내부 숫자 키 |
|---|---|---|
| 소환사 | `summoners.puuid` | `summoners.summoner_pk` |
| 경기 | `matches.match_id` | `matches.match_pk` |
| 경기 참가자 | `match_participants.match_participant_id` | `match_participants.match_participant_pk` |

기존 `summoners.id`는 Riot의 레거시 encrypted summoner ID이고, `match_participants.participant_id`는 경기 내 1~10 슬롯이므로 새 숫자 PK 이름으로 재사용하지 않는다.

## 전환 단계

1. **Foundation**
   - 부모 테이블에 nullable 숫자 키를 온라인 DDL로 추가한다.
   - 숫자 ID 매핑 테이블과 재시작 가능한 진행 상태를 만든다.
   - 신규 INSERT/UPDATE가 숫자 키를 즉시 갖도록 트리거를 설치한다.
   - 참가자가 소환사 저장보다 먼저 들어오는 기존 흐름에서는 `match_id`와 `puuid`로 부모 numeric identity를 선점하고, 이후 부모 행이 동일 identity를 재사용한다.
2. **Parent backfill**
   - 소환사, 경기, 참가자 순서로 작은 keyset 배치를 실행한다.
   - 작업 제한 시간이 지나면 cursor를 저장하고 정상 종료한다.
3. **Parent validation**
   - NULL, 중복, orphan, 기존 문자열 관계와 숫자 관계의 불일치를 0건으로 만든다.
   - 그 전에는 숫자 키를 읽기 경로에 사용하지 않는다.
4. **Child migration**
   - `masteries`, `summoner_matches`, 참가자 상세와 룬 등 하위 테이블에 숫자 FK를 추가한다.
   - 작은 하위 테이블은 이중 쓰기와 제한 UPDATE 백필 후 동일성 검증을 수행한다.
   - `masteries`는 3,500만 행을 in-place UPDATE한 뒤 다시 rebuild하지 않는다. 숫자 PK 기반 compact shadow table로 직접 복사해 숫자 FK 백필과 PK rebuild를 한 번에 처리한다.
5. **Read cutover**
   - 기능 단위 feature flag로 숫자 JOIN을 활성화하고 결과·성능을 비교한다.
   - 문제가 있으면 문자열 JOIN으로 즉시 복귀한다.
6. **Constraint cutover**
   - 저부하 점검 창에 NOT NULL, UNIQUE/PK, FK를 적용한다.
   - 필요한 임시 공간과 DDL 알고리즘을 운영에서 사전 확인한다.
7. **Legacy cleanup**
   - 한 릴리스 이상 안정화한 뒤 하위 테이블의 문자열 FK와 중복 인덱스를 제거한다.
   - 외부 식별자용 부모 테이블 문자열 UNIQUE는 유지한다.

## 운영 안전 조건

- #62 Champion Detail 초기 백필과 동시에 대형 DDL 또는 숫자 키 백필을 실행하지 않는다.
- 일반 백엔드 시작은 숫자 키 백필을 자동 실행하지 않는다.
- `backfill-numeric-keys` 유지보수 명령만 명시적으로 백필을 실행한다.
- 배치 크기와 실행 제한은 환경변수로 제한하고, 반복 실행해도 같은 결과가 나와야 한다.
- 전체 백필과 검증이 끝나기 전에는 PK 교체, 문자열 컬럼 삭제, 기존 인덱스 삭제를 금지한다.
- 운영 적용 전 `EXPLAIN`, 디스크 여유, metadata lock 대기, replica/binlog 영향을 확인한다.

## `masteries` 전용 shadow 전환

### 전환 이유

운영 `masteries`는 약 3,579만 행, 24.88GiB다. 기존 `(puuid, champion_id)` clustered PK의 긴 PUUID가 covering secondary index에도 row locator로 중복 저장된다. 기존 테이블의 `summoner_fk`를 전 행 UPDATE한 뒤 숫자 PK로 다시 rebuild하면 같은 대형 데이터를 두 번 쓰고 binlog·temporary space를 중복 소비한다.

따라서 generic child backfill은 `masteries`를 절대 UPDATE하지 않는다. `masteries_numeric_v2`에 숫자 키로 직접 복사한다.

### 목표 스키마

```sql
CREATE TABLE masteries_numeric_v2 (
    summoner_fk BIGINT UNSIGNED NOT NULL,
    champion_id BIGINT NOT NULL,
    champion_points_until_next_level BIGINT NOT NULL,
    chest_granted TINYINT(1) NOT NULL,
    last_play_time DATETIME NOT NULL,
    champion_level INT NOT NULL,
    champion_points INT NOT NULL,
    champion_points_since_last_level BIGINT NOT NULL,
    tokens_earned INT NOT NULL,
    PRIMARY KEY (summoner_fk, champion_id),
    KEY masteries_numeric_champion_points_level_covering_index
        (champion_id, champion_points DESC, champion_level)
) ENGINE=InnoDB;
```

외부 Riot 식별자인 PUUID는 `summoners`와 `summoner_numeric_keys`에 UNIQUE로 유지한다. 대용량 숙련도 행에는 저장하지 않는다. 최종 FK는 `summoners.summoner_pk`가 NOT NULL·UNIQUE/PK로 전환된 뒤 추가한다.

### 상태 머신

| 상태 | 의미 | 허용 작업 |
|---|---|---|
| `copying` | backend write가 중지되고 shadow schema에서 keyset copy 진행 중 | 재시작 가능한 bounded copy |
| `copied` | source cursor가 끝까지 도달함 | 전체 정합성 검증 |
| `validated` | source/shadow 행 수·aggregate checksum·orphan 검증 통과 | read shadow 비교 |
| `read_cutover` | 숫자 JOIN read 경로 검증 중 | 즉시 legacy read rollback 가능 |
| `write_cutover` | 애플리케이션이 numeric table에 직접 기록 | mirror trigger 제거 준비 |
| `retired` | 한 릴리스 안정화 후 legacy 제거 완료 | compact table만 유지 |

진행 상태는 별도 단일 행에 cursor `(puuid, champion_id)`, 누적 복사 행 수, 상태와 검증 시각을 저장한다. cursor와 복사 batch는 같은 트랜잭션에서 커밋한다.

### 쓰기 일관성

- 첫 구현은 안전성을 우선해 backend write를 중지한 maintenance window에서만 실행한다.
- 명시적 offline acknowledgement가 없으면 command가 시작을 거부한다.
- copy 중에는 mirror trigger를 설치하지 않는다. 전체 checksum 검증이 통과한 뒤에만 legacy `masteries`의 INSERT/UPDATE/DELETE를 shadow에 같은 트랜잭션으로 반영하는 sync trigger를 설치한다.
- copy 도중 실패해도 legacy table은 변경하지 않으며 저장된 cursor부터 재개한다.
- copy가 완료된 뒤에도 backend를 올리기 전에 consistent snapshot 검증과 sync trigger 설치를 마친다.
- 애플리케이션 쓰기는 당분간 legacy를 원본으로 유지하고 `MASTERY_READ_SOURCE=numeric_v2`일 때만 일반 조회와 숙련도 통계를 shadow로 전환한다. 문제가 있으면 `legacy`로 되돌리고 재시작하며, legacy 쓰기는 계속 최신 상태이므로 데이터 rollback이 필요하지 않다.
- 향후 legacy 제거 단계에서 application write를 numeric table로 전환하고 sync trigger를 제거한다.

### copy 및 검증

1. `masteries.puuid` 중 `summoner_numeric_keys`에 없는 값이 0건인지 확인한다.
2. 기존 PK `(puuid, champion_id)` 순서로 제한된 keyset batch를 읽는다.
3. 같은 범위만 shadow에 `INSERT ... SELECT`하고 cursor를 원자적으로 저장한다.
4. 실행 제한 시간이 끝나면 정상 종료하고 다음 실행에서 cursor 이후부터 재개한다.
5. source/shadow 행 수, champion별 행 수·점수 합계, 전체 aggregate checksum, NULL/orphan을 비교한다.
6. shadow 실제 크기와 primary/covering index page 수를 측정해 기존 24.88GiB와 비교한다.

### 운영 안전 경계

- 일반 backend 기동과 `migrate`는 shadow copy를 시작하지 않는다.
- 별도 명시적 maintenance command와 offline acknowledgement만 schema 준비·copy·검증을 실행한다.
- 기본 batch와 work limit에 상한을 두고, 디스크 12GiB 미만·lock wait·오류 발생 시 copy를 중단한다.
- binlog 비활성화는 replica 0건과 별도 사용자 승인을 동시에 확인한 운영 copy session에서만 허용한다. 기본값은 binlog 기록이다.
- 검증 전 rename, legacy drop, feature flag 기본값 변경을 금지한다.
- `MASTERY_READ_SOURCE=numeric_v2`로 기동할 때 schema, copy 완료, validated 상태와 sync trigger 3개를 모두 확인하고 하나라도 없으면 backend 시작을 거부한다.

### 격리 MySQL 측정 결과

- MySQL 8.0.20의 100만 행 fixture에서 제한 실행 후 cursor 재개, 완료 후 멱등 재실행, advisory lock, checksum 손상 탐지와 복구를 검증했다.
- legacy 테이블은 275,775,488 bytes, numeric shadow는 112,492,544 bytes로 data와 index 합계가 59.21% 감소했다.
- 첫 1,000행 제한 실행은 2.645초, 나머지 999,000행 copy+검증은 1분 43.481초였고 처리율은 약 9,650행/초였다.
- 운영 3,579만 행의 단순 환산은 약 62분이지만 실제 maintenance 계획은 운영 I/O, checksum scan, binlog 증가량과 12GiB 디스크 안전선을 반영해 보수적으로 수립한다.
- 격리 검증은 운영 DB에 변경을 적용하지 않았으며 기존 `masteries`를 UPDATE하지 않았다.

## 단계별 완료 기준

Foundation은 다음 조건을 만족하면 완료다.

- 매핑·진행 테이블과 부모 숫자 컬럼이 존재한다.
- 신규 쓰기 동기화 트리거가 존재한다.
- 로컬 MySQL에서 신규·기존 행 백필과 재실행을 검증한다.

Task #64 전체 완료는 다음 조건을 모두 만족해야 한다.

- 부모와 범위 내 하위 테이블의 숫자 키 백필 및 일치 검증이 완료된다.
- 숫자 JOIN 읽기 경로가 운영 검증을 통과한다.
- 최종 PK/FK 전환의 실행 시간·잠금·디스크 사용량과 rollback 절차가 기록된다.
- 문자열 키를 하위 PK·FK·보조 인덱스에서 제거한 뒤 회귀 테스트를 통과한다.
