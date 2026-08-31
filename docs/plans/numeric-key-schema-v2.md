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
2. **Parent backfill**
   - 소환사, 경기, 참가자 순서로 작은 keyset 배치를 실행한다.
   - 작업 제한 시간이 지나면 cursor를 저장하고 정상 종료한다.
3. **Parent validation**
   - NULL, 중복, orphan, 기존 문자열 관계와 숫자 관계의 불일치를 0건으로 만든다.
   - 그 전에는 숫자 키를 읽기 경로에 사용하지 않는다.
4. **Child migration**
   - `masteries`, `summoner_matches`, 참가자 상세와 룬 등 하위 테이블에 숫자 FK를 추가한다.
   - 이중 쓰기와 배치 백필 후 동일성 검증을 수행한다.
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
