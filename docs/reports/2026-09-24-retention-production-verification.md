# #76 Retention production verification

| 항목 | 결과 |
|---|---|
| 일시 | 2026-09-24 17:12~17:20 KST |
| 배포 | `0194c6c` 백엔드 이미지, `e5516b0` preview-only 스케줄러 |
| 영향 범위 | 백엔드만 교체·재시작; 리플레이 서버는 변경 없음 |
| 자동 실행 | cron 미설치, `.env.retention` 미생성; 기본 비활성 |

## 검증 결과

| 단계 | 관측 |
|---|---|
| 배포 전 | 백엔드 healthy, 디스크 여유 약 39 GiB |
| 운영 dry-run | 최신 8개 패치 16.10~16.17 보존, 만료 버전 없음, 대상 경기 0건 |
| 스케줄러 preview-only | 운영 Linux/Python 3.9·Docker Compose 연결 성공, 대상 0건, 백엔드 중단 없음 |
| 제한된 실제 명령 | 백엔드 중단 후 `batchSize=10`, `batchTimeout=30s`, `workLimit=1m`로 실행; 대상·삭제 모두 0건, 443ms에 정상 완료 |
| 복구 | 백엔드 healthy, 기본 및 챔피언·메타 통계 API HTTP 200 |
| 디스크 | 최종 여유 약 40 GiB |

실제 행 삭제 및 삭제 도중 실패 후 복구는 운영 데이터에 만료 패치가 없어 검증하지 못했다. 보존 정책을 임의로 7개 패치 이하로 낮춰 대상 데이터를 만들지 않았다.

## 검증 중 발생한 중단

첫 번째 no-op 실행 시 Windows PowerShell에서 원격 `bash -s`로 전달한 여러 줄 스크립트의 BOM/줄 끝 문자가 명령 인식에 영향을 줬다. `set -e`가 적용되지 않았고 `cleanup-retention` 대신 임시 컨테이너가 일반 서버로 기동되었다. 이 컨테이너에서는 삭제 로직이 실행되지 않았다. 정확한 임시 컨테이너만 중지한 뒤 백엔드를 복구했고, healthy와 HTTP 200을 확인했다. 이후 따옴표가 필요 없는 함수형 EXIT trap을 무해한 명령으로 먼저 검증하고, 한 줄 SSH 명령으로 no-op 실제 실행을 완료했다.

향후 Windows에서 여러 줄 스크립트를 SSH 표준입력으로 전달하는 방식은 이 절차에 사용하지 않는다. 운영 중단·복구 명령은 원격 단일 셸에서 EXIT trap과 명시적인 timeout을 함께 사용한다.

## 남은 조건

- 만료 패치가 실제로 생겼을 때 작은 배치의 삭제·재실행 및 실패 복구를 관측한다.
- 초기 검증 시점에는 운영자 확인 전이어서 자동 실행과 삭제 활성화를 설정하지 않았다. 이후 활성화 결과는 아래에 기록한다.

## 2026-09-24 17:40 KST 운영 활성화

운영자 요청으로 기본 간격을 7일로 바꾸고 `a9fa34c`의 systemd service/timer를 설치했다. 운영 설정은 최신 8개 패치, KST 02:00~04:00, 최소 디스크 여유 20GiB, DB 실행 스레드 상한 8·락 대기 0, 배치 100건·2분 및 전체 10분 제한이다. `.env.retention`은 Git에 포함되지 않고 권한 600으로 보관한다.

cron 패키지는 설치돼 있지 않아 systemd timer를 사용한다. timer는 활성화됐고 다음 확인 시각은 2026-09-24 18:00 KST이다. 창 밖 수동 실행에서 `outside_window`와 service 성공을 확인했다. 실행 계정 `ec2-user`의 Docker 그룹과 백엔드 재시작 후 healthy·API 200도 확인했다. 웹훅 주소가 없어 오류 알림은 현재 journal 확인만 가능하다. 실제 삭제는 대상이 생길 때 검증하며 #76은 `🟣 VFY`로 유지한다.

## 2026-10-02 03:00 KST 첫 실제 삭제

2026-10-02 16:47 KST에 journal, scheduler 상태, 서비스 상태를 읽기 전용으로 확인했다. 운영 호스트 checkout은 `02d832b`이다.

| 항목 | 관측 |
|---|---|
| preview | 보존 16.12~16.19, 만료 16.10·16.11의 6개 full version, 대상 21,433경기 |
| 삭제 | 2,500경기, 611초, `completed=false`, `stopReason=work_limit`, 잔여 18,933경기 |
| 주요 삭제 행 | 참가자·상세·perks 각 28,024, perk style 56,048, perk selection 168,144, 밴 23,694, 팀 5,000, summoner_matches 2,696, DataExplorer 처리 상태 2,425 |
| 복구 | 삭제 종료 18초 뒤 `backend_restored`; 확인 시점 backend healthy, `/`·champion·meta-summary API 200 |
| 상태 파일 | `completedAt=2026-09-25 02:00`, `attemptedAt=2026-10-02 03:00`; 24시간 재시도 간격 뒤 재개 |
| 디스크 | 루트 여유 36GiB (InnoDB 파일은 삭제만으로 줄지 않음) |

삭제 이후 백엔드 로그에서 15:31 KST에 숙련도 통계 수집의 `driver: bad connection` 1건을 확인했다. 5분 뒤 재시도하는 일시 오류로 삭제 작업과의 연관은 확인되지 않았다.

실행당 약 2,500경기 처리 속도를 유지하면 잔여 분량은 약 8회의 야간 실행(회당 백엔드 중단 약 10분)이 더 필요하다.

## 2026-10-02 18:14 KST 운영 DB 조치

| 항목 | 결과 |
|---|---|
| buffer pool | `innodb_buffer_pool_size` 128MiB → 384MiB, `SET PERSIST` 온라인 확장 완료, 재시작 후에도 유지 |
| 메모리 판단 | RAM 3.8GiB, 가용 약 2.1GiB, swap 2GiB 사용 중이라 1GiB 대신 384MiB로 제한 |
| 영향 확인 | backend healthy, `/`·champion·meta-summary API 200, mysqld active |
| 정리 | 과거 통합 테스트가 남긴 `teamgg_retention_test_*` 픽스처 DB 2개 삭제 |

같은 MySQL 인스턴스를 `mips` 스키마가 함께 사용하므로 buffer pool 변경은 해당 스키마에도 적용된다.

## 2026-10-03 03:00 KST 첫 online 실행

| 항목 | 관측 |
|---|---|
| 대상 | 전날 남은 18,933경기 (16.10·16.11의 5개 full version) |
| 결과 | 18,933경기 전부 삭제, `completed=true`, 2,786초(약 46분), 분당 약 408경기 |
| 안전장치 | `throttledWaits=0`, `retriedBatches=0`, 백엔드 중단 없음 |
| 주요 삭제 행 | 참가자·상세·perks 각 217,962, perk style 435,924, perk selection 1,307,772, 밴 185,222, 팀 37,866, summoner_matches 20,795 |
| 서비스 | 실행 구간 오류는 숙련도 통계 `driver: bad connection` 1건(전날 삭제 없는 시간에도 발생), backend healthy 유지 |
| 디스크 | 루트 여유 35GiB |
| 상태 파일 | `completedAt=2026-10-03 03:00`; 다음 실행은 7일 뒤 |

## 2026-10-03 16:03 KST 운영 알림 검증

Discord 웹훅(`/slack` 호환 주소)을 `.env.retention`에 설정하고 `52b913d`(명시적 User-Agent)를 서버에 반영했다. 운영 호스트에서 직접 POST와 스케줄러 `alert()`로 테스트 알림을 보내 HTTP 200을 확인했다. #76을 DONE 처리했다.
