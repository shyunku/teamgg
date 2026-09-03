# 통계 마이그레이션 및 운영 성능 검증 보고서

- 검증 일시: 2026-08-30 08:27 ~ 진행 중 KST
- 대상: backend Task #61, #62
- 운영 배포 최종 커밋: `af16a54`
- 운영 환경: Docker Compose backend, MySQL 8

## 1. 요약

Task #61 숙련도 통계는 운영 마이그레이션과 첫 수집을 완료했다. 약 3,533만 숙련도 행 전체를 매번 정렬하던 구조를 dirty champion 기반 materialized aggregate로 변경했고, 236개 챔피언 초기 갱신을 1분 25초, 184개 후속 갱신을 1분 31초에 처리했다. Top 30도 챔피언별 covering-index 조회로 바뀌어 filesort가 사라졌다.

Task #62 챔피언 상세·메타 통계는 단일 대형 staging INSERT를 폐기하고 경기 단위 cursor, 처리 완료 마커, 실행 시간 제한을 사용하는 증분 영구 source로 전환했다. 운영 초기 백필은 장기 트랜잭션 없이 약 60초 작업 단위로 재개되고 있으며 최신 패치 `16.17`을 완료한 뒤 `16.16`을 처리 중이다. 최종 snapshot과 API 갱신은 백필 완료 후 별도로 검증한다.

## 2. 적용 변경

### 2.1 공통 마이그레이션

- `schema_migrations` 기반 순차 마이그레이션을 운영에서 실행했다.
- `20260830_001`, `20260830_002`, `20260830_003`을 포함한 마이그레이션이 모두 `dirty=0`인 상태로 일반 백엔드의 `validate` 기동을 통과했다.
- 백엔드 일반 기동은 `DB_MIGRATION_MODE=validate`로 스키마 검증을 통과했다.
- Task #61 통계 테이블 2개와 Task #62 증분 source 테이블 4개 및 view를 운영에 적용했다.
  - `mastery_statistics_aggregates`
  - `mastery_statistics_dirty_champions`
  - `champion_detail_statistics_participants`
  - `champion_detail_statistics_bans`
  - `champion_detail_statistics_processed_matches`
  - `champion_detail_statistics_progress`
  - `champion_detail_statistics_valid_builds` view

### 2.2 Task #61 숙련도

- `masteries(champion_id, champion_points DESC, champion_level)` covering index를 추가했다.
- INSERT/UPDATE/DELETE 후 dirty champion을 기록하는 트리거 3개를 추가했다.
- 전체 GROUP BY 대신 dirty champion별 bounded aggregate 갱신으로 전환했다.
- Top 30은 materialized champion key set을 읽고 챔피언별 index lookup을 수행하도록 변경했다.
- Riot 데이터에 포함된 `600xx` 비챔피언 숙련도 ID가 전체 수집을 실패시키지 않도록 현재 Data Dragon에 존재하는 챔피언만 공개 통계에 포함했다. 원본 행은 삭제하지 않았다.

### 2.3 Task #62 챔피언 상세·메타

- 최근 3개 short patch에 속하는 full version만 대상으로 삼는다.
- 단일 대형 `champion_detail_statistics_source` INSERT를 경기 ID cursor 기반 작은 배치로 교체했다.
- 참가자·밴·처리 완료 경기·버전별 cursor를 각각 영구 저장해 프로세스가 종료되어도 다음 실행에서 이어서 처리한다.
- cursor보다 과거에 늦게 유입된 경기는 processed-match anti join으로 다시 찾아 누락을 방지한다.
- source 준비 작업에 실행 시간 제한을 두고, 제한에 도달하면 현재 배치까지 커밋한 뒤 글로벌 통계 락을 반환한다.
- source가 완성되기 전에는 기존 snapshot을 유지하고, 완성된 후에만 base·position·meta·counter 집계와 snapshot 교체를 수행한다.
- one-shot 명령 `docker compose run --rm backend collect-champion-detail`을 추가해 일반 서버·DataExplorer loop 없이 수집만 검증할 수 있게 했다.
- 배치 후보 `250`, `10`, `1000`과 match lookup index 선택을 운영에서 비교했다.

## 3. 장애와 조치

### 3.1 `Temporary file write failure` (Error 1878)

초기 인덱스 생성은 MySQL `tmpdir`이 약 1.9GiB tmpfs인 `/tmp`를 사용해 실패했다. 운영 설정을 다음 영구 디스크 경로로 변경한 뒤 재실행했다.

- `tmpdir=/var/lib/mysql/tmp`
- `innodb_tmpdir=/var/lib/mysql/tmp`

변경 후 covering index 생성과 마이그레이션이 진행됐다. 검증 종료 시 루트 볼륨은 128GB 중 약 94GB 사용, 약 35GB 여유였다.

### 3.2 information_schema trigger 매핑 오류

MySQL 드라이버가 `EVENT_OBJECT_TABLE`, `ACTION_TIMING`, `EVENT_MANIPULATION`을 대문자 label로 반환해 sqlx의 소문자 struct tag 매핑이 실패했다. 컬럼명 기반 struct 매핑 대신 `QueryRowxContext(...).Scan(...)` 위치 기반 스캔으로 수정했다.

### 3.3 비챔피언 숙련도 ID

운영 `masteries`에는 `60001`부터 `60117` 범위의 비챔피언 ID가 다수 존재했다. 기존 코드는 첫 ID에서 전체 수집을 실패시켰다. 해당 ID는 warning 후 통계 결과에서 제외하고 정상 챔피언 snapshot 생성을 계속하도록 변경했다.

## 4. 운영 측정 결과

### 4.1 데이터 규모

| 항목 | 운영 추정값 |
| --- | ---: |
| `masteries` | 35,330,564행 |
| `match_participants` | 11,276,579행 |
| `match_participant_details` | 8,439,558행 |
| `matches` | 874,236행 |
| `match_teams` | 1,580,390행 |

### 4.2 숙련도 전후 비교

| 지표 | 기존 | 변경 후 |
| --- | --- | --- |
| 집계 범위 | 약 3,533만 행 전체 순회·정렬 | dirty champion만 갱신 |
| 초기 갱신 | 전체 스캔 | 236개, 1분 25.87초 |
| 후속 갱신 | 전체 스캔 | 184개, 1분 31.15초 |
| 대표 Top 30 | 약 42만 행 범위 정렬 | champion index lookup, filesort 없음 |
| snapshot | 과거 snapshot 유지 | JSON 1,029,783B, gzip 484,293B 저장 성공 |

`EXPLAIN`에서 aggregate 쿼리는 covering index만 사용했다. Top 30은 `champion_id` ref access와 `champion_points DESC` 인덱스 순서를 사용했고 `using_filesort=false`였다.

활성 DataExplorer가 수집 직후에도 숙련도 행을 추가하므로 대표 챔피언 81은 materialized 286,906명, live 286,911명으로 5행 차이가 발생했다. 같은 챔피언이 dirty queue에 즉시 기록된 것을 확인했으며 이는 설계된 eventual consistency 동작이다.

### 4.3 챔피언 상세·메타 비교

| 실행 | 관찰 시간 | 결과 |
| --- | ---: | --- |
| 운영 구버전 기준 측정 | 546초 이상 | 미완료 상태 관찰 |
| staging 배포 후 기존 base 쿼리 | 1,560초 | 미완료, 안전 중단 |
| 최근 패치 인덱스 + 조인 제거 base 쿼리 | 917초 | 미완료, 안전 중단 |
| source-first staging INSERT | 909초 | 미완료, 안전 중단 |

base 쿼리는 관찰 기준 1,560초에서 917초로 최소 41.2% 짧아졌지만 완료 시간은 확보하지 못했다. 따라서 이 값은 완료 성능 개선율이 아니라 동일 운영 환경에서의 중단 시점 비교다.

#### 수정 1차 — 최근 패치 선필터와 대형 영구 staging

| 항목 | 결과 |
| --- | --- |
| 상황 | 전체 원본 테이블을 매 실행마다 임시 테이블로 복제·정렬했다. |
| 변경 | 최근 3개 패치를 먼저 고르고 영구 staging source를 만든 뒤 base·position·meta·counter를 계산하도록 분리했다. |
| 성능 변화 | base 관찰 중단 시점은 1,560초에서 917초로 짧아졌지만 완료되지 않았다. source INSERT도 909초 동안 완료되지 않았다. |
| 판단 | 단일 대형 INSERT와 장기 트랜잭션 자체가 운영 규모에 맞지 않아 폐기했다. |

#### 수정 2차 — 재시작 가능한 증분 source

| 항목 | 결과 |
| --- | --- |
| 상황 | 1차 구조는 중단하면 처음부터 다시 시작해 운영 배포와 검증이 불가능했다. |
| 변경 | 경기 ID cursor, processed-match 마커, 버전별 진행 상태, 늦게 유입된 과거 경기 fallback, 실행 시간 제한을 추가했다. |
| 성능 변화 | 약 60초 단위로 수백 경기를 커밋하고 다음 주기에서 이어서 처리한다. `16.17` 완료 후 `16.16`으로 전환됨을 확인했다. |
| 안정성 변화 | 장기 단일 트랜잭션이 사라졌고, 검증 중 deadlock·임시 파일 오류·중복 키 실패는 발생하지 않았다. |
| 판단 | 초기 백필 시간이 남더라도 중단·재배포·재시작이 가능한 구조이므로 채택했다. |

#### 수정 3차 — 배치 250에서 10으로 축소

| 배치 | 운영 관찰값 | 해석 |
| ---: | --- | --- |
| 250 | 500~750경기 / 약 66~73초 | 배치 하나가 길고 작업 제한을 크게 넘길 수 있었다. |
| 10 | 대표 구간 695~777경기 / 약 60초, 데이터 구간에 따라 1,299경기 / 60초 | 작은 배치가 더 빠르고 제한 시간 준수와 중단 응답성도 좋았다. |

기본값을 `10`으로 변경했다. 한 번의 SQL이 작아져 장애 시 롤백 범위도 줄었다.

#### 수정 4차 — source batch의 match PK lookup 강제

| 항목 | 결과 |
| --- | --- |
| 변경 | 이미 선택한 match ID 집합을 다시 읽는 참가자·밴·processed-marker 쿼리에 `PRIMARY` lookup을 강제했다. |
| 운영 관찰 | 690~736경기 / 약 60초로, 변경 전 대표 범위와 유의미한 차이가 없었다. |
| 판단 | 실행계획 안정성 회귀 테스트는 유지하되 속도 개선 효과로는 계산하지 않는다. |

#### 수정 5차 — 배치 1000 비교 실험

| 항목 | 결과 |
| --- | --- |
| 조건 | batch `1000`, work limit `2m`, 외부 timeout `5m` |
| 운영 관찰 | 2개 배치 1,011경기, 2분 21.14초, 약 430경기/분 |
| 성능 변화 | batch 10의 낮은 대표값 695경기/분과 비교해도 약 38% 느렸다. 단일 배치 때문에 2분 work limit도 21초 초과했다. |
| 판단 | 처리량과 중단 응답성이 모두 악화돼 폐기하고 기본값 10을 유지했다. |

#### 수정 6차 — 최종 백필·snapshot 검증

사용자가 허용한 약 1시간 점검 창에는 일반 backend를 중지하고 DataExplorer 경쟁 없이 one-shot만 실행했다. batch는 `10`, work limit은 `10m`, 구간 재시도는 `1s`로 두었다.

| 집중 구간 | 처리 경기 | 시간 | 처리량 |
| ---: | ---: | ---: | ---: |
| 1차 | 11,820 | 10분 0.07초 | 약 1,182경기/분 |
| 2차 | 11,810 | 10분 0.34초 | 약 1,181경기/분 |
| 3차 | 11,810 | 10분 0.50초 | 약 1,180경기/분 |
| 4차 | 11,620 | 10분 0.14초 | 약 1,162경기/분 |
| 5차 | 11,090 | 10분 0.37초 | 약 1,108경기/분 |
| 합계 | 58,150 | 약 50분 | 평균 약 1,163경기/분 |

일반 운영의 대표 695~777경기/분보다 약 50~67% 빠르다. API backend와 DataExplorer를 내리고 bounded 구간 사이 휴지를 15초에서 1초로 줄인 효과다. MySQL은 2 vCPU 호스트에서 약 119~130% CPU를 사용했고, 루트 볼륨은 77% 사용·약 31GB 여유를 유지했다. deadlock, duplicate key, temporary file write failure, 수집 오류는 발생하지 않았다.

점검 창 종료 시 source는 `16.16.804.9184`, cursor `KR_8348493336`까지 진행했으나 전체 대상 full version은 아직 준비되지 않았다. one-shot을 grace 종료하고 일반 backend를 복구했으며, 완료 배치는 보존되고 진행 중 배치만 롤백된다. 따라서 base·position·meta·counter 최종 집계, 새 snapshot 및 API `updatedAt` 갱신 검증은 background 백필 완료 후 이어서 기록한다.

복구 후 backend는 `validate` 기동과 health check를 통과했고 공개 API 3개가 모두 HTTP 200을 반환했다. 10분 initial delay 뒤 첫 background 구간이 `602경기 / 60.42초`를 처리해 cursor `KR_8349189095`로 이어졌으며, 이후에도 cursor가 `KR_8350045991`까지 전진했다. 수동 종료 전 완료 배치와 재시작 cursor가 정상 보존되고 글로벌 통계 락이 정상 해제됐음을 확인했다.

#### 수정 7차 — background 백필 완료 확인

2026-08-31 16:09 KST에 운영 상태를 다시 읽기 전용으로 확인했다. 대상 full version 6개의 진행 상태가 모두 `completed=1`이었고 처리 경기 합계는 500,470건이었다.

| full version | 처리 경기 | 완료 |
| --- | ---: | ---: |
| `16.17.810.4348` | 30,904 | 1 |
| `16.16.804.9184` | 198,119 | 1 |
| `16.15.802.4387` | 144,360 | 1 |
| `16.15.801.3452` | 92,783 | 1 |
| `16.15.800.8073` | 19,768 | 1 |
| `16.15.799.6036` | 14,536 | 1 |

source 완료 뒤 base·position·meta·counter 집계와 shared snapshot 교체도 완료됐다. champion 및 meta-summary API가 동일한 새 `updatedAt` `2026-08-30T18:30:56.081618598Z`(KST 2026-08-31 03:30)를 반환했으며, 현재 수집 loop는 snapshot이 아직 fresh하다고 판단해 만료 시점까지 재수집을 skip한다. 이로써 초기 백필, 최종 집계, 공개 API 갱신과 반복 실행 skip 검증을 모두 충족했다.

첫 최적화 실행 직전과 모든 검증 종료 후의 MySQL 전역 지표는 다음과 같다. 같은 기간 DataExplorer가 계속 실행되어 통계 작업 단독 수치로 해석할 수 없다.

| 지표 | 직전 | 종료 후 | 변화 |
| --- | ---: | ---: | ---: |
| `Created_tmp_tables` | 51 | 96 | +45 |
| `Created_tmp_disk_tables` | 19 | 49 | +30 |
| `Innodb_row_lock_waits` | 99 | 154 | +55 |
| `Innodb_row_lock_time` | 578,452ms | 845,425ms | +266,973ms |

## 5. 최종 운영 상태

- backend 컨테이너: healthy
- replay-analyzer 컨테이너: healthy
- 일회성 검증 컨테이너: 제거 완료
- 마이그레이션: `20260830_003`을 포함해 backend `validate` 기동 통과
- Champion Detail 증분 source: 대상 full version 6개, 총 500,470경기 백필 완료
- 공개 `/`, champion, meta-summary API: HTTP 200
- 공개 Champion Detail snapshot: `2026-08-30T18:30:56.081618598Z`로 갱신 완료
- 원본 경기·참가자·숙련도 데이터 삭제 없음

## 6. 결론 및 후속 작업

Task #61은 운영 배포·수집·snapshot·실행계획 검증을 완료했다.

Task #62는 작은 배치, 재시작 가능 cursor, 처리 완료 마커, 실행 시간 제한을 갖춘 증분 source로 전환해 기존 장기 대형 쿼리 문제를 제거했다. 운영 집중 검증에서도 약 50분 동안 오류 없이 58,150경기를 처리했다.

이후 background 백필이 대상 full version 6개와 총 500,470경기를 모두 완료했고 base·position·meta·counter 집계, 새 snapshot/API 갱신, fresh snapshot 재실행 skip을 확인했다. 따라서 Task #62의 운영 완료 조건을 모두 충족해 DONE으로 전환한다.
