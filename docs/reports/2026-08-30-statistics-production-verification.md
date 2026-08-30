# 통계 마이그레이션 및 운영 성능 검증 보고서

- 검증 일시: 2026-08-30 08:27 ~ 11:38 KST
- 대상: backend Task #61, #62
- 운영 배포 최종 커밋: `fca3331`
- 운영 환경: Docker Compose backend, MySQL 8

## 1. 요약

Task #61 숙련도 통계는 운영 마이그레이션과 첫 수집을 완료했다. 약 3,533만 숙련도 행 전체를 매번 정렬하던 구조를 dirty champion 기반 materialized aggregate로 변경했고, 236개 챔피언 초기 갱신을 1분 25초, 184개 후속 갱신을 1분 31초에 처리했다. Top 30도 챔피언별 covering-index 조회로 바뀌어 filesort가 사라졌다.

Task #62 챔피언 상세·메타 통계는 영구 staging과 최근 패치 선필터를 운영 배포했으나 완료 조건을 충족하지 못했다. base 집계는 개선 전 1,560초, 조인 최적화 후에도 917초 동안 완료되지 않았다. staging source INSERT도 909초 동안 완료되지 않아 두 작업 모두 안전하게 중단했다. 다음 단계는 패치·챔피언 단위 증분 영구 집계와 재시작 가능한 bounded backfill이다.

## 2. 적용 변경

### 2.1 공통 마이그레이션

- `schema_migrations` 기반 순차 마이그레이션을 운영에서 실행했다.
- `20260830_001`, `20260830_002`를 포함한 12개 마이그레이션이 모두 `dirty=0`임을 확인했다.
- 백엔드 일반 기동은 `DB_MIGRATION_MODE=validate`로 스키마 검증을 통과했다.
- 신규 통계 테이블 3개를 확인했다.
  - `mastery_statistics_aggregates`
  - `mastery_statistics_dirty_champions`
  - `champion_detail_statistics_source`

### 2.2 Task #61 숙련도

- `masteries(champion_id, champion_points DESC, champion_level)` covering index를 추가했다.
- INSERT/UPDATE/DELETE 후 dirty champion을 기록하는 트리거 3개를 추가했다.
- 전체 GROUP BY 대신 dirty champion별 bounded aggregate 갱신으로 전환했다.
- Top 30은 materialized champion key set을 읽고 챔피언별 index lookup을 수행하도록 변경했다.
- Riot 데이터에 포함된 `600xx` 비챔피언 숙련도 ID가 전체 수집을 실패시키지 않도록 현재 Data Dragon에 존재하는 챔피언만 공개 통계에 포함했다. 원본 행은 삭제하지 않았다.

### 2.3 Task #62 챔피언 상세·메타

- 최근 3개 short patch의 full version만 선필터하는 영구 staging source를 추가했다.
- 기존 24개 명시적 임시 테이블을 staging 기반 CTE 메타·카운터 집계로 교체했다.
- base/position 쿼리가 `matches_game_version_index`를 사용하도록 조인 순서를 고정했다.
- 승패 합계를 위해 불필요하게 사용하던 `match_teams` 조인을 제거하고 `match_participants.win`을 사용했다.
- base, position, source, meta, counter 단계별 duration 로그를 추가했다.
- source 준비를 첫 단계로 옮겨 staging 비용을 독립적으로 관찰할 수 있게 했다.

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

첫 최적화 실행 직전과 모든 검증 종료 후의 MySQL 전역 지표는 다음과 같다. 같은 기간 DataExplorer가 계속 실행되어 통계 작업 단독 수치로 해석할 수 없다.

| 지표 | 직전 | 종료 후 | 변화 |
| --- | ---: | ---: | ---: |
| `Created_tmp_tables` | 51 | 96 | +45 |
| `Created_tmp_disk_tables` | 19 | 49 | +30 |
| `Innodb_row_lock_waits` | 99 | 154 | +55 |
| `Innodb_row_lock_time` | 578,452ms | 845,425ms | +266,973ms |

## 5. 최종 운영 상태

- backend 컨테이너: healthy
- 실행 중인 Champion Detail 통계 SQL: 0개
- 일회성 검증 컨테이너: 제거 완료
- `champion_detail_statistics_source`: 0행. 중단된 INSERT는 커밋되지 않았다.
- 마이그레이션: 12개 모두 clean
- 원본 경기·참가자·숙련도 데이터 삭제 없음

## 6. 결론 및 후속 작업

Task #61은 운영 배포·수집·snapshot·실행계획 검증을 완료했다.

Task #62는 쿼리 구조와 관측성은 개선했지만 운영 규모에서 완료되지 않아 WIP를 유지한다. 다음 구현은 다음 조건을 만족해야 한다.

1. 패치·챔피언 단위 영구 aggregate를 사용해 전체 최근 패치 재집계를 제거한다.
2. 신규 경기만 반영하는 dirty queue 또는 cursor 기반 증분 갱신을 적용한다.
3. 최초 backfill은 작은 배치, 재시작 가능 cursor, 실행 시간 제한을 제공한다.
4. source 생성도 단일 대형 INSERT가 아니라 패치 또는 match PK 범위별 bounded batch로 나눈다.
5. DataExplorer와 통계 backfill의 동시 I/O를 제한하거나 별도 저부하 창에서 실행한다.
6. 동일 운영 기준에서 완료 시간, 임시 공간, row lock, snapshot 갱신을 다시 측정한다.
