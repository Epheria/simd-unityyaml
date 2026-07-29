# 설계 노트 (ko)

## 전제: 왜 "Unity 방언 한정"인가

풀 YAML의 SIMD 구조 파싱이 불가능하다는 것이 커뮤니티 정설이다 — 들여쓰기가
의미를 갖고, `key:`의 콜론을 보기 전까지 스칼라인지 매핑 시작인지 알 수 없어
무제한 lookahead가 필요하며, 따옴표 없는 스칼라 때문에 문맥 의존성이 강하다.
(YAML 스펙 분량은 JSON의 ~12배.)

Unity 직렬화 방언은 예외다:

- **엄격한 라인 기반** — 구조 경계가 항상 개행과 일치
- **고정 문서 헤더** `--- !u!<classID> &<fileID>[ stripped]`
- 들여쓰기 규칙적(2칸), 거대 인라인 flow(`{x: 0, y: 0}`)도 라인 안에 갇힘

따라서 simdjson stage 1의 핵심(구조 문자 위치의 벡터 일괄 추출)이 이 subset에서
성립한다: 개행 위치 스캔이 곧 구조 복원의 8할이다.

## 테이프 설계 (v0.1)

```
Tape{ LineStarts []uint32, Docs []Doc, N int }
```

- 오프셋은 uint32 — 4 GiB 초과 입력은 명시적 에러(ErrTooLarge). 조용한 절단 금지.
- `Docs`는 파일 순서. 헤더 prefix가 맞지만 나머지가 안 풀리는 라인은
  `Malformed: true`로 **보고**한다(버리지도, 고쳐 추측하지도 않음) — unity-lens의
  the trust rule을 라이브러리 계약으로 옮긴 것.

## 커널 구조

```
kernel_arm64.s      NEON: 64B 블록당 4×VCMEQ + 비트가중 VAND + VADDP×3 (simdjson
                    neon movemask-bulk 형태) → 블록당 uint64 마스크
kernel_generic.go   SWAR 폴백: 정확한 zero-byte 검출(Hacker's Delight 변형).
                    주의 — (x-lo)&^x&hi 는 borrow 오탐이 있어 쓰지 않는다(실제로
                    개발 중 fuzz 이전 랜덤 테스트에서 잡힌 버그).
kernel_arm64.go     디스패치: full 블록은 NEON, 꼬리는 generic
```

- **cgo 없음** — Go 어셈블리(.s)만. 크로스 컴파일과 소비자의 툴체인 무결성 유지.
- arm64는 ASIMD가 ARMv8 baseline 필수라 런타임 감지 불필요.

## 검증 전략 (이 프로젝트의 trust rule)

1. `newlineMasksNaive`(바이트 루프)가 모든 커널의 오라클 — 고정/랜덤/비정렬 차등 테스트
2. `indexReference`(마스크 없이 순수 스칼라로 Index 재구현)와 `FuzzIndex` 차등 fuzz —
   SIMD 경로는 스칼라 진실과 bit-identical해야 한다
3. 벤치는 `b.SetBytes`로 MB/s 공시, 실파일은 `UYAML_BENCH_FILE`로

## 실측 (Apple M4 Pro, 8 MiB 합성 입력)

- NEON 개행 스캔 ~75 GB/s, SWAR ~7.6 GB/s (9.9배)
- Index end-to-end: NEON 3.1 GB/s vs 순수 스칼라 1.8 GB/s (1.7배)
  → Amdahl: 이제 병목은 마스크 소비(테이프 빌드)와 헤더 파싱. stage 2에서
  콜론 스캔을 벡터화할 때 같은 마스크 파이프라인을 재사용한다.

## 정직한 포지셔닝 (README에도 명시)

- 다수의 작은 파일 워크로드에서는 syscall이 병목(unity-lens 실측: 64%)이라
  이 라이브러리의 이득이 ~5% 상한. **대형 단일 버퍼**(수 MB 씬, mmap, 서버
  파이프라인)가 진짜 무대다.
- 벤치 비교 시 stage 1(구조 인덱스)은 풀 파스보다 일이 적다는 점을 항상 병기.
  rapidyaml ~200 MB/s와의 비교는 참고선이지 동일 작업 비교가 아니다.

## 로드맵

1. v0.2: 라인 내 key-콜론 위치 스캔(두 번째 벡터화 대상), `m_Name`/`fileID:`/
   `guid:` 참조 추출용 stage 2 테이프
2. AVX2 커널(amd64) — SWAR 대비 검증된 이득이 있을 때만
3. unity-lens 백엔드 채택 실험 — 단일 대형 scene `read` 경로 한정으로 계측 후 판단
