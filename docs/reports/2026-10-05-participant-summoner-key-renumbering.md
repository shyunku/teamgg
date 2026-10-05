# Participant summoner key renumbering

| 항목 | 내용 |
|---|---|
| 발견 | 2026-10-04 00:30 KST #70 전체 정합성 검증 |
| 발생 | 2026-08-31 15:52~17:56 KST (DB 기록 기준) |
| 영향 | `match_participants` 1,193행(PUUID 1,052개)과 `match_participant_numeric_keys` 1,193행의 `summoner_fk`가 존재하지 않는 소환사 숫자 ID를 가리킴 |
| 서비스 영향 | 없음. API는 아직 문자열 키로 JOIN한다. #72 숫자 JOIN 전환 전에 발견했다. |
| 조치 | 2026-10-05 19:06 KST 데이터 수정, `35b66d2` 코드 수정 |

## 현상

- 참가자 행과 참가자 매핑 행이 같은 소환사 ID(예: 286420)를 갖고 있었지만, 그 ID는 `summoner_numeric_keys`에 없었다.
- 같은 PUUID의 현재 키는 더 큰 ID(예: 402824)였고, 키 행의 `created_at`은 참가자 매핑 생성 시각과 마이크로초까지 같았다. 키 행이 만들어진 뒤 ID만 바뀐 것이다.
- 같은 PUUID의 참가 기록이 옛 ID와 현재 ID로 나뉘어 있었다.
- `leagues`, `summoner_matches`, `masteries_numeric_v2`, 참가자 `match_fk`에는 같은 문제가 없었다.

## 원인

부모 숫자 키 백필(`backfillSimpleNumericKeys`)이 identity 행을 다음처럼 upsert했다.

```sql
INSERT INTO summoner_numeric_keys (puuid) SELECT puuid FROM summoners WHERE puuid IN (...)
ON DUPLICATE KEY UPDATE summoner_id = VALUES(summoner_id)
```

PUUID 행이 이미 있으면 `VALUES(summoner_id)`는 이 INSERT가 새로 발급하려던 auto-increment 값이 된다. 그래서 참가자 trigger가 먼저 만들어 참가자에게 준 ID가 백필 시점에 새 ID로 바뀌었다. 2026-08-31 오후는 소환사 부모 키 백필을 운영에서 처음 실행한 시간대다. 그 뒤로는 백필 대상 소환사가 없어 재발하지 않았다.

처음에는 trigger의 `LAST_INSERT_ID()`가 동시 롤백 때문에 잘못된 값을 남긴 것으로 추정했다. 하지만 로컬 재현에 실패했다. 대신 동시 쓰기 통합 테스트에 identity 테이블 비교 검증을 추가하자, 기존 trigger 그대로에서 백필 upsert만 바꾸면 통과하고 되돌리면 실패했다. 이로써 원인을 확정했다.

## 조치

| 구분 | 내용 |
|---|---|
| 코드 | 중복 시 identity 컬럼 대신 Riot 식별자 컬럼에 no-op 갱신(`35b66d2`) |
| 검증 강화 | 참가자 백필 검증이 매핑 테이블뿐 아니라 소환사·경기 identity 테이블과도 직접 비교 |
| 데이터 | 1,193행을 PUUID의 현재 ID로 한 트랜잭션에서 갱신. 참가자 1,193행·매핑 1,193행 갱신, 4초 |
| 재검증 | 참가자 ↔ 소환사 identity 불일치 0건, 참가자 ↔ 매핑 불일치 0건, backend healthy, 관련 오류 없음 |
| 백업 | 갱신 전 값(`match_id`, `participant_id`, `match_participant_id`, 옛 ID, 새 ID)을 서버 `/home/ec2-user/task70-fix-backup-20261005.tsv`에 보관 |

## 작업 중 교훈

- `summoner_numeric_keys.puuid`(ascii_bin)와 `match_participants.puuid`(utf8mb4)는 collation이 달라 조인 순서에 따라 실행 계획이 크게 달라진다. 첫 수정 시도에서 `CREATE ... SELECT`와 매핑 UPDATE가 각각 10분 넘게 걸려 중단했다(롤백 확인, 데이터 변경 없음). 이후 해시 조인이 되는 조회로 대상을 파일로 뽑고, ascii_bin 키를 가진 임시 테이블로 인덱스 조인해 수정했다.
- `pkill -f`의 패턴이 같은 SSH 명령줄과 겹쳐 자기 셸을 종료한 일이 있었다. 원격에서 프로세스를 종료할 때는 `^bash /tmp/...`처럼 고정된 패턴을 쓴다.
