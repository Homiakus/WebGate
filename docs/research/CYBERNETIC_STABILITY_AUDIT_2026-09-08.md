# WebGate — математический и кибернетический аудит устойчивости

**Дата:** 2026-09-08  
**Статус:** normative research input for planning  
**Область:** browser/runtime, transport, relay/origin, gateway, authorization, process management, persistence, release/update chain  
**Связанный план:** `MASTER_PLAN.md`, `docs/implementation/CYBERNETIC_STABILITY_PROGRAM.md`

---

## 1. Цель аудита

WebGate рассматривается не как набор сервисов и прокси, а как распределённая кибернетическая система с несколькими контурами управления, общими ресурсными ограничениями и жёсткими security-инвариантами.

Ключевой вывод: основной риск проекта находится не в одном конкретном алгоритме и не в криптографии как таковой, а в **взаимодействии нескольких независимых state machine**:

- browser/session lifecycle;
- transport readiness/failover;
- relay/origin routing;
- authorization;
- process lifecycle;
- durable state;
- release/update verification.

Каждый локальный автомат может быть корректен, но композиция может породить глобально неустойчивое поведение: cascading failure, ложную readiness, lock-in в `Offline`, head-of-line blocking, exhaustion ресурсов, routing ambiguity и расхождение между фактическим и заявленным состоянием продукта.

Поэтому дальнейшая архитектура должна строиться вокруг четырёх принципов:

1. **Evidence-based state** — состояние допустимо только при наличии положительного доказательства.
2. **Bounded resources** — ни один пользователь/stream/relay не может создать неограниченную работу.
3. **Explicit authority/routing** — маршрут и право доступа никогда не выводятся из «первого доступного» состояния.
4. **Single source of state transition semantics** — один authoritative автомат на каждый класс управления, без дублирования правил переходов в разных слоях.

---

# 2. Системная модель

Определим глобальное состояние WebGate:

\[
X(t)=[M,R,Q,N,L,E,\Phi,C,A,V,P]
\]

где:

- \(M\) — глобальный operating mode;
- \(R\) — routing state;
- \(Q\) — вектор очередей;
- \(N\) — число активных соединений/streams/tasks;
- \(L\) — latency vector;
- \(E\) — error-rate vector;
- \(\Phi\) — suspicion/liveness state;
- \(C\) — доступная вычислительная/сетевой ёмкость;
- \(A\) — authorization state;
- \(V\) — verified release/config epoch;
- \(P\) — active policy epoch.

Управляющее воздействие:

\[
U(t)=[admit,route,failover,shed,recover,revoke]
\]

Возмущения:

\[
W(t)=[traffic,loss,RTT,attacker,relayFailure,originFailure,authorityFailure]
\]

Целевая функция эксплуатации:

\[
J=w_L P99Latency+w_D DropRate+w_S SwitchRate+w_R ResourceUse+w_A Unavailability
\]

при жёстких ограничениях безопасности:

\[
DirectEgressAllowed=0
\]

\[
Q_i \le Q_{i,max}
\]

\[
N_i \le N_{i,max}
\]

\[
Route(request)=AuthorizedOrigin(request)
\]

\[
Open \Rightarrow RendererProof
\]

\[
VerifiedRelease \Rightarrow SignatureValid \land DigestValid \land RollbackSafe
\]

Security-инварианты не являются штрафами в функции качества. Нарушение любого из них переводит состояние в недопустимое.

---

# 3. P0: release verification не образует криптографическую границу доверия

Файл: `crates/webgate-core/src/release.rs`.

Текущий `ReleaseManifest::verify_artifact()` принимает `artifact_bytes` и внешний `actual_sha256_hex`, но фактически:

- не вычисляет SHA-256 непосредственно из `artifact_bytes` внутри verifier;
- сравнивает manifest digest со строкой, переданной вызывающей стороной;
- принимает signature как присутствующую, если `signature_hex` не пуста;
- не выполняет реальную Ed25519 verification в этой функции.

Это означает, что функция с семантикой «verify authenticity» пока не создаёт доверенную verification boundary.

## Требуемая модель

Verifier обязан сам вычислять:

\[
d=SHA256(artifact)
\]

Затем формировать однозначно канонизированный signed message:

\[
m=CanonicalEncode(schema,version,platform,sourceCommit,d,size,keyId,epoch)
\]

и проверять:

\[
Ed25519Verify(pk_{release},m,\sigma)=true
\]

Release может перейти в состояние `VERIFIED` только если:

\[
SignatureValid \land DigestValid \land PlatformValid \land RollbackSafe
\]

## Архитектурное требование

- digest не принимается из внешней стороны как доверенный факт;
- public key выбирается из versioned trust store по `key_id`;
- подписывается canonical binary/structured representation, а не неканонический JSON;
- ключи имеют rotation/revocation policy;
- release verification использует широко проверяемую криптографическую библиотеку, а не собственную реализацию как единственную production boundary.

**Severity:** P0 / release blocker.

---

# 4. P0: relay identity и secure envelope

Файлы:

- `server/pkg/relay/relay.go`;
- `server/pkg/origin/agent.go`;
- `server/cmd/webgate-relay/main.go`;
- `crates/webgate-transport/src/relay.rs`.

Текущий Go relay использует WGRL framing поверх TCP и общий `ClusterToken` для Origin authentication. Rust `SecureRelayTransport` при этом честно остаётся fail-closed и сообщает, что secure relay backend ещё не реализован.

## Риск

Один shared bearer secret создаёт широкую область компрометации:

\[
Compromise(ClusterToken) \Rightarrow ImpersonationRisk(cluster)
\]

Identity должна быть привязана к уникальному криптографическому ключу узла:

\[
Identity_{node}=PublicKey_{node}
\]

## Требуемая архитектура

Production Origin↔Relay и Client↔Relay link:

- TLS 1.3 или QUIC/TLS;
- mTLS / per-node certificate identity;
- unique `key_id`/certificate на каждый Relay и Origin;
- rotation/revocation;
- cluster token допускается только как bounded bootstrap/recovery mechanism, не как конечный root of trust;
- identity из transport handshake должна совпадать с explicit route identity.

Инвариант:

\[
CertIdentity(connection)=Route.origin
\]

**Severity:** P0.

---

# 5. P0: explicit routing вместо first-active-origin

Файл: `server/pkg/relay/relay.go`.

`getActiveOrigin()` выбирает первый доступный origin из map. Это неприемлемо для multi-origin/multi-tenant системы.

Требуемая функция:

\[
Route:(Tenant,Cluster,Origin,Service,Session)\rightarrow OriginConnection
\]

а не:

\[
Route=FirstAlive(map)
\]

Нужен explicit routing table, обновляемый атомарно. Каждому client stream до выделения ресурсов должен соответствовать route reservation.

Минимальный маршрут:

```text
RouteKey {
  tenant_id
  cluster_id
  origin_id
  service_id
}
```

Для stream требуется:

```text
reservation_id
route_epoch
expires_at
max_streams
max_bytes
```

При замене routing table старые streams могут продолжить существование только по документированной epoch policy; новые streams обязаны использовать новый route epoch.

**Severity:** P0.

---

# 6. P0/P1: head-of-line blocking в relay multiplexing

Текущая модель имеет общий connection writer (`writeMu`) и единый frame read loop, который отправляет payload в per-stream buffered channel.

Пусть для stream \(s\):

\[
\lambda_s > \mu_s
\]

где \(\lambda_s\) — скорость поступления данных, \(\mu_s\) — скорость потребления.

При размере буфера \(B\):

\[
T_{fill}\approx \frac{B}{\lambda_s-\mu_s}
\]

После заполнения channel единый read loop блокируется. Тогда один медленный stream может сделать:

\[
\mu_{all}\rightarrow 0
\]

для всей origin connection.

## Требуемая модель

Предпочтительно transport-level multiplexing через QUIC streams:

```text
secure QUIC connection
├── control stream
├── app stream A
├── app stream B
└── app stream C
```

Если WGRL/1 сохраняется:

```text
network reader
   ↓
bounded dispatcher
   ↓
per-stream bounded queues
   ↓
DRR / WFQ scheduler
```

Overflow одного stream:

\[
Overflow(stream_i)\Rightarrow Reset(stream_i)
\]

но никогда:

\[
Overflow(stream_i)\Rightarrow Stall(connection)
\]

**Severity:** P0 для публичного relay plane, P1 для ограниченного pilot.

---

# 7. P0/P1: failover должен стать единым гибридным автоматом

Файлы:

- `crates/webgate-transport/src/failover.rs`;
- `crates/webgate-transport/src/dual_failover.rs`.

Сейчас присутствуют два набора семантики failover: модельный `TransportFailoverController` и собственная параллельная логика внутри `DualRelayFailoverTransport`.

Это создаёт опасность semantic drift.

## Целевая state machine

\[
M \in \{PRIMARY,PRIMARY\_DEGRADED,FALLBACK,FALLBACK\_DEGRADED,RECOVERING,OFFLINE\}
\]

Для каждого path:

\[
x_i=[L_i,E_i,\Phi_i,S_i]
\]

где:

- \(L_i\) — EWMA latency;
- \(E_i\) — EWMA error rate;
- \(\Phi_i\) — liveness suspicion;
- \(S_i\) — saturation.

Обновление:

\[
E_t=\alpha e_t+(1-\alpha)E_{t-1}
\]

\[
L_t=\beta l_t+(1-\beta)L_{t-1}
\]

Risk score:

\[
R_i=w_EE_i+w_L\max(0,\frac{L_i-L_{target}}{L_{target}})+w_\Phi\Phi_i+w_SS_i
\]

Failover при:

\[
R_i>\theta_{off}
\]

Switchback только если:

\[
R_i<\theta_{on}
\]

с hysteresis:

\[
\theta_{on}<\theta_{off}
\]

Дополнительно:

- `OFFLINE` не должен быть поглощающим состоянием;
- recovery probes работают из `OFFLINE` и `FALLBACK`;
- probe traffic и real traffic observations различаются;
- failures классифицируются как local-connect / upstream / authorization / service / saturation;
- переключение одного path не должно автоматически изменять глобальную доступность без достаточного evidence.

---

# 8. P1: unbounded concurrency и thread-per-connection

В Rust transport proxy на каждое соединение создаётся OS thread и `JoinHandle` сохраняется до shutdown.

Для активных соединений по закону Литтла:

\[
N_{active}=\lambda_{conn}E[T_{conn}]
\]

Для накопленных handles:

\[
N_{handles}(t)\approx\int_0^t \lambda_{conn}(\tau)d\tau
\]

Даже после завершения соединения handle остаётся в контейнере до stop, что превращает длительно работающий процесс в систему с монотонно растущим bookkeeping state.

## Требование

- async runtime / bounded task executor;
- global connection semaphore;
- per-device/per-tenant/per-origin quotas;
- bounded accept backlog;
- early rejection/load shedding;
- completed task handles немедленно reap/remove;
- метрики `active`, `queued`, `rejected`, `saturated`.

Инварианты:

\[
N_{active}\le N_{max}
\]

\[
Q\le Q_{max}
\]

При достижении лимита:

\[
RejectEarly > QueueUnbounded
\]

---

# 9. P1: положительная обратная связь cascading failure

Опасный контур:

```text
load ↑
  ↓
threads/queues ↑
  ↓
latency ↑
  ↓
health score worsens
  ↓
failover
  ↓
remaining capacity ↓
  ↓
utilization ↑
  ↓
latency ↑↑
  ↓
reconnect/retry ↑
```

То есть возникает положительная обратная связь:

\[
Load\rightarrow Latency\rightarrow Failover\rightarrow ReducedCapacity\rightarrow Load
\]

Поэтому admission control должен быть реализован раньше, чем дальнейшее усложнение failover.

Целевой порядок реакции:

1. detect saturation;
2. shed excess work;
3. preserve existing healthy sessions;
4. degrade optional functions;
5. only then alter route when failure evidence указывает именно на path, а не на локальную перегрузку.

---

# 10. P1: canonical destination policy должна быть единой

Файлы:

- `crates/webgate-core/src/policy.rs`;
- `crates/webgate-transport/src/socks5_proto.rs`;
- `crates/webgate-transport/src/restricted_http_connect.rs`;
- `server/pkg/domain/service.go`.

Сейчас URL/domain canonicalization и wildcard semantics реализованы несколькими различными алгоритмами.

Это создаёт parser differential risk:

\[
Policy_A(raw)\ne Policy_B(raw)
\]

Целевой pipeline:

```text
raw input
  ↓
canonical parser
  ↓
CanonicalDestination
  ├── Browser policy
  ├── HTTP CONNECT policy
  ├── SOCKS policy
  └── Gateway routing
```

`CanonicalDestination` должен содержать уже нормализованные:

```text
scheme
host / IP
port
path
service_id
route identity
```

Никакой downstream layer не должен повторно интерпретировать raw URL.

---

# 11. P1: unknown HTTP methods должны fail closed

Файл: `server/pkg/gateway/gateway.go`.

Сейчас неизвестный method отображается на `PermView`.

Это небезопасно для сервисов, поддерживающих WebDAV или другие state-changing methods.

Инвариант:

\[
UnknownMethod\Rightarrow DENY
\]

Лучший контракт:

\[
(service,route,method)\rightarrow RequiredPermission
\]

Service registration должна явно декларировать разрешённые methods или использовать безопасный default allowlist.

---

# 12. P1: secret inheritance должен быть allowlist-based

Файл: `server/pkg/process/manager.go`.

Сейчас child environment очищается от отдельных известных секретов blacklist-методом. Это хрупко: появление нового control secret автоматически наследуется дочерними приложениями, если разработчик не обновит blacklist.

Целевая модель:

\[
Env_{child}=AllowList(required\ variables)
\]

Всё остальное не передаётся.

Отдельный `SecretBroker` должен выдавать только capability-scoped ephemeral credentials, если сервису действительно нужен секрет.

---

# 13. P1: health должен измерять полезную функцию, а не только handshake

SOCKS greeting или наличие TCP connection подтверждает лишь L1/L2 состояние.

Нужны уровни:

```text
L1 process alive
L2 transport handshake works
L3 relay ↔ origin route is live
L4 authorized synthetic service request succeeds
```

Глобальный `Ready` должен требовать свежего evidence как минимум уровня, соответствующего обещаемой функции.

Нельзя считать:

\[
SOCKSHandshakeOK\Rightarrow ServiceReady
\]

Нужно:

\[
Ready(service,path)\Rightarrow Fresh(L3/L4Evidence)
\]

---

# 14. P1: availability нельзя считать без common-mode failures

Для независимых relay:

\[
A_{dual}=1-(1-A_1)(1-A_2)
\]

Но при общей вероятности common-mode failure \(c\):

\[
A_{dual}\approx(1-c)[1-(1-A'_1)(1-A'_2)]
\]

Поэтому production HA должна учитывать корреляцию:

- provider;
- ASN/network;
- region;
- DNS dependency;
- transport family;
- code version;
- authority dependency;
- signing/control plane;
- shared secret/root identity.

Нужна `FailureDomainVector` для каждого path и автоматическая проверка, что primary/fallback действительно независимы по заданным критериям.

---

# 15. P1: request-time SecureAcces как availability bottleneck

Синхронная authorization RPC на каждый request сохраняет правильный fail-closed semantics, но превращает authority в последовательный availability dependency:

\[
A_{request}\approx A_{relay}A_{origin}A_{gateway}A_{authority}A_{service}
\]

Опциональный future mode может использовать короткоживущий подписанный grant:

\[
G=Sign_{Auth}(device,session,resource,permissions,expiry,policyEpoch)
\]

Gateway проверяет grant локально.

Это допустимо только при:

- коротком TTL;
- explicit expiry;
- policy epoch;
- key rotation;
- bounded offline mode;
- невозможности бесконечного cache extension.

Fail-open запрещён.

---

# 16. P1: timeout semantics должны быть фазовыми

Один wall-clock timeout на всю операцию плохо подходит для downloads, SSE, WebSocket, длинных ответов и AI-интерфейсов.

Нужна модель:

\[
T=T_{connect}+T_{TLS}+T_{headers}+T_{idle}
\]

с независимыми лимитами.

`idle timeout` должен означать отсутствие прогресса, а не максимальную жизнь корректного stream.

---

# 17. P2: persistence serialization

SQLite configuration (`WAL`, `synchronous=FULL`, `trusted_schema=OFF`) разумна для безопасного control plane.

При этом single DB connection и persistence внутри registry lock дают:

\[
T_{lock}\ge T_{commit}
\]

На текущем масштабе это приемлемо как deliberate simplicity, но должно измеряться.

Необходимо:

- histogram commit latency;
- lock hold duration;
- mutation throughput;
- explicit SLO для control plane;
- при необходимости command serialization через один durable writer вместо удержания domain lock на I/O.

Обычный SHA-256 checksum записи защищает от случайной corruption, но не является доказательством аутентичности против процесса с write-доступом к БД.

---

# 18. WGBR: правильное направление при жёстком ограничении scope

`MASTER_PLAN` уже содержит переход к WebGate Browser Runtime с zero-network renderer и capability IPC.

Это сильный архитектурный ход, потому что:

\[
RendererNetworkCapability=0
\]

проще доказать, чем бесконечный набор сетевых deny-rules.

Однако WebGate не должен пытаться стать универсальным браузером.

Нужно определить `WGWeb Profile` — ограниченный совместимый поднабор:

- нужные HTML primitives;
- нужный CSS subset;
- конкретные JS/Web APIs;
- navigation contract;
- storage contract;
- fetch/capability contract;
- accessibility contract.

Каждое расширение WGWeb проходит compatibility/security budget.

Иначе размер состояния приблизительно растёт как:

\[
|S_{system}|\sim\prod_i |S_i|
\]

и формальная проверяемость теряется.

---

# 19. Целевой Safety Kernel

WebGate следует свести к небольшому числу authoritative kernels.

## 19.1 Policy Kernel

Единственный canonical parser и decision engine.

```text
RawIntent
  ↓
CanonicalIntent
  ↓
PolicyDecision
```

## 19.2 Route Kernel

```text
AuthorizedIntent
  ↓
Explicit Route + epoch + reservation
```

## 19.3 Transport Supervisor

```text
observations
  ↓
health estimator
  ↓
state machine
  ↓
route/path actuation
```

## 19.4 Resource Governor

```text
admission
quota
queue bounds
fair scheduling
load shedding
```

## 19.5 Release Trust Kernel

```text
artifact bytes
manifest
trust store
  ↓
cryptographic verification
  ↓
verified release epoch
```

## 19.6 Renderer Capability Kernel

Renderer не получает socket/DNS/file/process capability напрямую.

---

# 20. Формальные инварианты для model checking

Минимальный набор для TLA+/PlusCal или эквивалентной проверки:

### Safety

```text
I-CYB-001 DirectEgressNeverAllowed
I-CYB-002 UnauthorizedRouteNeverSelected
I-CYB-003 RendererOpenRequiresPositiveProof
I-CYB-004 ReleaseVerifiedRequiresRealCryptoProof
I-CYB-005 QueueAndConcurrencyAreBounded
I-CYB-006 OneStreamCannotBlockUnrelatedStreamsIndefinitely
I-CYB-007 RevokedDeviceCannotCreateNewAuthorizedStream
I-CYB-008 RouteEpochCannotCrossTenant
I-CYB-009 UnknownMethodCannotGainImplicitPermission
I-CYB-010 ChildProcessCannotInheritControlSecretsByDefault
```

### Liveness

```text
L-CYB-001 OfflineCanRecoverWhenAQualifiedPathReturns
L-CYB-002 FallbackEventuallyReturnsToHealthyPrimaryUnderStableConditions
L-CYB-003 DeadStreamsAreEventuallyReaped
L-CYB-004 ExpiredReservationsAreEventuallyFreed
L-CYB-005 CompletedTasksDoNotAccumulateUnboundedBookkeepingState
```

---

# 21. Требуемые проверки

## Concurrency

- Rust `loom` для критических state transitions;
- Go `-race`;
- deterministic scheduler tests где возможно;
- deadlock/lock-order tests.

## Property/Fuzz

- canonical URL equivalence;
- malformed WGRL frames;
- routing epoch replacement;
- duplicate stream IDs;
- stream-ID wraparound;
- malformed release manifest;
- version comparison/pre-release cases.

## Chaos

- packet loss;
- RTT spikes;
- half-open TCP;
- relay restart;
- origin restart;
- authority stall;
- disk stall;
- exhausted FD;
- exhausted memory budget;
- one intentionally slow stream among many healthy streams.

## Load

Проверять систему до и после saturation, а не только throughput при нормальной нагрузке.

Обязательные графики:

```text
arrival rate
accepted rate
rejected rate
active streams
queue depth
P50/P95/P99 latency
failover count
CPU/RSS/FD
```

---

# 22. Новая иерархия приоритетов

## P0 — до production pilot

1. Настоящая cryptographic release verification.
2. Native secure relay envelope + per-node identity.
3. Explicit relay routing.
4. Admission controller + hard resource bounds.
5. Устранение connection-wide HOL failure semantics.
6. Единственный authoritative failover supervisor.
7. Правдивый release/readiness status в документации.

## P1 — до controlled production

1. Unified canonical destination model.
2. Unknown HTTP methods fail closed.
3. Child environment allowlist.
4. Multi-level health model.
5. Failure-domain correlation model.
6. Phase-specific timeout semantics.
7. Optional signed authorization lease model.
8. Long-duration soak/chaos qualification.

## P2 — после стабилизации data plane

1. Persistence throughput optimization.
2. Advanced path scoring.
3. Predictive capacity control.
4. Sophisticated multipath scheduling.

---

# 23. Release gate после аудита

WebGate не должен называться production-ready, пока открыт любой P0 из этого аудита.

Формально:

\[
ProductionReady \Leftrightarrow
P0_{open}=0
\land ReleaseBinaryQualification
\land SecurityInvariantsPass
\land SoakPass
\land ChaosPass
\]

Наличие успешной компиляции, unit-тестов или исторически закрытых фаз не эквивалентно production readiness.

---

# 24. Главная архитектурная формула

Целевая WebGate architecture должна минимизировать не количество ошибок по отдельности, а размер области опасных состояний:

\[
\boxed{
CapabilityIsolation
+BoundedResources
+EvidenceBasedState
+ExplicitRouting
+FormalInvariants
}
\]

Это должно стать общей математической основой WGBR, transport, relay, SecureAcces integration и ContinuitySession.
