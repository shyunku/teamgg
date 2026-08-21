결론부터 말하면, **현재 스키마와 수집 구조 모두 용량 증가에 비효율적인 부분이 큽니다.** 특히 스키마보다 더 큰 원인은 DataExplorer가 사실상 무제한으로 사용자 관계망을 탐색하는 구조입니다. 분석만 수행했으며 DB나 코드는 변경하지 않았습니다.

## 현재 용량 구성

| 영역                 | 대략적인 크기 |
| -------------------- | ------------: |
| `masteries`          |          12GB |
| 경기·참가자·룬 계열  |          24GB |
| DataExplorer 큐/출처 |         1.3GB |
| 기타 테이블          |      1GB 내외 |
| InnoDB 임시 공간     |         7.8GB |
| MySQL binlog         |        2.75GB |

### 주요 테이블

| 테이블                                    |  행 추정 | 데이터 | 인덱스 |
| ----------------------------------------- | -------: | -----: | -----: |
| `masteries`                               |  2,929만 |  7.6GB |  4.6GB |
| `match_participants`                      | 약 987만 |  5.6GB |  2.5GB |
| `match_participant_perk_style_selections` |  3,843만 |  2.9GB |  3.0GB |
| `match_participant_perk_styles`           |  1,230만 |  2.2GB |  2.8GB |
| `match_participant_details`               | 약 712만 |  3.0GB |  613MB |
| `data_explorer_summoner_jobs`             | 약 186만 |  437MB |  496MB |

## 1. 가장 큰 원인: 무제한 DataExplorer 확장

신규 소환자를 처리할 때 다음 작업을 합니다.

- 숙련도 전체 저장
- 최근 경기 3개 조회
- 각 경기 참가자 전원을 다시 소환자 탐색 큐에 추가

관련 코드는 [data_explorer.go](/home/ec2-user/workspace/teamgg/apps/backend/service/data_explorer.go:315)와 [core.go](/home/ec2-user/workspace/teamgg/apps/backend/service/core.go:376)에 있습니다.

`depth` 컬럼은 저장되지만 최대 깊이를 검사하는 코드가 없습니다. 결과적으로:

```text
소환자
 └─ 최근 경기 3개
     └─ 경기당 참가자 약 10명
         └─ 각 참가자의 최근 경기 3개
             └─ 다시 모든 참가자...
```

중복은 어느 정도 막지만 KR 전체 사용자 관계망 방향으로 계속 확장됩니다.

현재 설정의 일일 상한은:

- 신규 소환자 처리: 10,000명
- 신규 경기 처리: 30,000건

현재 평균 저장량을 기준으로 모두 신규 데이터라면 하루 최대 증가량은 대략:

- 숙련도: 약 500MB/일
- 경기·참가자·룬: 약 1.1~1.2GB/일
- 큐와 인덱스: 추가 증가

즉, **최대 약 1.5~1.7GB/일 증가할 수 있는 구조**입니다.

가장 먼저 해야 할 조치는 다음 중 하나입니다.

- DataExplorer 일시 중단
- 탐색 깊이 제한 추가
- 경기 참가자 전원을 자동 탐색하는 로직 제거
- 등록 사용자 또는 명시적으로 요청된 사용자만 수집
- 데이터 보존 기간 설정

## 2. 문자열 UUID를 하위 테이블 전체에 반복

`match_participant_id`와 `style_id`는 매번 36자리 UUID로 생성됩니다.

- 참가자 UUID 생성: [core.go](/home/ec2-user/workspace/teamgg/apps/backend/service/core.go:291)
- 스타일 UUID 생성: [core.go](/home/ec2-user/workspace/teamgg/apps/backend/service/core.go:448)

이 UUID가 참가자 상세, 룬, 룬 스타일, 룬 선택과 각 인덱스에 반복됩니다.

특히 룬 계열:

- styles: 약 5GB
- selections: 약 5.9GB
- stat perks: 약 700MB
- 합계: 약 11.6GB

Riot 룬 구조는 사실상 고정된 형태입니다.

- primary style 1개
- sub style 1개
- primary perks 4개
- sub perks 2개
- stat perks 3개

따라서 별도의 UUID 기반 3개 테이블보다 참가자 테이블 또는 `participant_perks` 한 행에 고정 컬럼으로 저장하는 편이 훨씬 효율적입니다. 이 구조로 옮기면 룬 영역에서만 **8~10GB 정도 절감 가능성**이 있습니다.

## 3. `masteries`의 긴 PUUID 복합 PK

PUUID 실제 길이는 전부 78자입니다. 그런데 스키마는 `VARCHAR(255) utf8mb4`이고 다음 복합 PK를 사용합니다.

```sql
PRIMARY KEY (puuid, champion_id)
```

InnoDB 보조 인덱스는 클러스터 PK를 함께 저장하므로 `(champion_id, champion_points)` 인덱스에도 78자리 PUUID가 반복됩니다. 이 인덱스 하나가 4.56GB입니다.

권장 구조:

```text
summoners
  id BIGINT PK
  puuid VARCHAR(78) ASCII UNIQUE

masteries
  summoner_id BIGINT
  champion_id SMALLINT UNSIGNED
  ...
  PK (summoner_id, champion_id)
```

이 변경만으로 `masteries`에서 수 GB를 줄일 수 있습니다. `champion_id`가 `BIGINT`인 것도 과도하며 `SMALLINT UNSIGNED`면 충분합니다.

또한 숙련도 갱신은 API가 반환한 모든 챔피언을 개별 UPSERT합니다. [mastery.go](/home/ec2-user/workspace/teamgg/apps/backend/models/mastery.go:20) 현재는 삭제 정책이 없어 플레이어와 챔피언 조합이 계속 누적됩니다.

## 4. 인덱스 낭비

MySQL이 2026-03-24에 시작된 이후 사용 횟수를 확인한 결과:

- `match_participant_perk_styles(description)`: 0회, 약 1.1GB
- `match_participants(participant_id)`: 0회, 약 239MB
- `match_participants(team_position)`: 0회, 약 314MB
- `match_participants(champion_id)`: 0회, 약 280MB

특히 `description`은 `primaryStyle`과 `subStyle` 정도의 저선택도 값이며, 실제 통계 코드는 전체 데이터를 임시 테이블에 옮긴 다음 CASE 문으로 사용합니다. [champion_detail_statistics_meta.go](/home/ec2-user/workspace/teamgg/apps/backend/models/mixed/statistics_models/champion_detail_statistics_meta.go:77)

따라서 `description` 인덱스는 우선 삭제 후보입니다. 나머지도 쿼리 실행 계획을 최종 검증한 뒤 제거하면 약 800MB 이상 추가 절감할 수 있습니다.

## 5. PK가 없는 대형 테이블

### 룬 선택 테이블

[scheme.ddl](/home/ec2-user/workspace/teamgg/apps/backend/scheme.ddl:161)의 `match_participant_perk_style_selections`에는 PK가 없습니다.

그 결과:

- InnoDB 숨은 `GEN_CLUST_INDEX`: 약 2.9GB
- `style_id` FK 인덱스: 약 3.0GB

38백만 행에 대해 데이터와 문자열 인덱스가 거의 같은 크기로 중복됩니다. 고정 컬럼으로 평탄화하는 것이 최선이고, 유지한다면 적어도 정수형 participant/style 키와 명시적인 PK가 필요합니다.

### `summoner_matches`

저장소의 DDL에는 `(puuid, match_id)` PK가 있지만 [scheme.ddl](/home/ec2-user/workspace/teamgg/apps/backend/scheme.ddl:376), 운영 DB에는 실제 PK가 없습니다. 전용 마이그레이션은 존재하지만 [20260720_add_data_explorer_queue.sql](/home/ec2-user/workspace/teamgg/apps/backend/migrations/20260720_add_data_explorer_queue.sql:7) 적용되지 않은 상태로 보입니다.

코드는 `NOT EXISTS`로 중복을 우회하지만 [summoner_match.go](/home/ec2-user/workspace/teamgg/apps/backend/models/summoner_match.go:14), 동시 요청에서는 중복이 생길 수 있습니다.

이는 코드 DDL과 실제 운영 스키마가 어긋난 상태이므로, 스키마 버전 관리 도입이 필요합니다.

## 6. 완료된 DataExplorer 작업을 영구 보관

작업 완료 시 행을 삭제하지 않고 `status='done'`으로만 변경합니다. [data_explorer_job.go](/home/ec2-user/workspace/teamgg/apps/backend/models/data_explorer_job.go:346)

현재:

- summoner jobs: 약 932MB
- match jobs: 약 130MB
- match sources: 약 260MB

큐가 계속 커지는 구조입니다. 완료 후 일정 기간이 지나면 삭제하거나, 큐에는 pending/processing만 두고 마지막 탐색 시각을 `summoners`에 기록하는 편이 낫습니다.

단, 현재 로직에서 곧바로 완료 행만 삭제하면 재등록과 재처리가 늘 수 있으므로 탐색 제한 로직을 먼저 수정해야 합니다.

## 7. 통계 집계가 임시 공간과 DB 부하를 크게 발생

점검 시 숙련도 통계 쿼리가 약 **83분째 실행 중**이었습니다. 이 작업은 2,900만 개 숙련도 행을 12시간마다 전부 집계합니다. [mastery_statistics.go](/home/ec2-user/workspace/teamgg/apps/backend/models/mixed/statistics_models/mastery_statistics.go:20)

현재 인덱스에는 `champion_level`이 없어 집계 중 클러스터 레코드 조회가 대량 발생합니다. 다음과 같은 커버링 인덱스 또는 증분 집계가 필요합니다.

```sql
(champion_id, champion_points DESC, champion_level)
```

Champion Detail 통계는 여러 단계의 `CREATE TEMPORARY TABLE ... SELECT`와 정렬·윈도 함수를 실행합니다. [champion_detail_statistics_meta.go](/home/ec2-user/workspace/teamgg/apps/backend/models/mixed/statistics_models/champion_detail_statistics_meta.go:17) 이 작업들이 7.8GB InnoDB 임시 공간의 주원인입니다.

임시 공간은 MySQL 재시작으로 축소할 수 있지만, 쿼리를 바꾸지 않으면 다시 커집니다.

## 권장 우선순위

1. **DataExplorer를 일시 중단하거나 예산을 크게 낮추기**
2. 참가자 전원 재탐색 제거 또는 `maxDepth` 적용
3. 83분 이상 실행되는 mastery 통계 중단 및 쿼리 개선
4. 미사용 `description` 인덱스 제거
5. 완료된 DataExplorer 큐 정리 정책 구현
6. 누락된 `summoner_matches` PK 마이그레이션 적용
7. `summoners.id`, `matches.id`, `participants.id` 기반 숫자 FK로 스키마 v2 설계
8. 룬 계열을 참가자당 한 행으로 평탄화
9. 오래된 경기 데이터 보존·아카이브 정책 도입

가장 효과가 큰 것은 **DataExplorer 확장 차단**과 **숫자 키 기반 스키마 v2**입니다. 현 구조를 그대로 두고 인덱스 몇 개만 제거하는 것으로는 증가 속도를 잡기 어렵습니다.
