# WebGate

**WebGate** — защищенный клиент и шлюз управления частными ресурсами с изоляцией на уровне приложения (`Application-Scoped Access`) без изменения общесистемных сетевых маршрутов (`OS default route untouched`).

Основной сценарий взаимодействия:

```text
Telegram / доверенная ссылка
        ↓
     WebGate
        ↓
WebGate-owned protected browser/runtime
        ↓
Локальный destination-restricted transport boundary
        ↓
Отказоустойчивый слой релеев / VPS
        ↓
Приватный сервер приложений + SecureAcces
```

По умолчанию общесистемные сетевые маршруты операционной системы остаются нетронутыми. Только трафик защищенной browser/runtime-капсулы WebGate должен проходить через WebGate transport boundary.

> **Важно о текущем статусе:** WebGate находится в активной фазе security/resilience convergence. Репозиторий содержит значительную часть реализованного фундамента и release-candidate сборки, но проект **не считается production-qualified**, пока остаются открытые P0-блокеры из `MASTER_PLAN.md` и программы `docs/implementation/CYBERNETIC_STABILITY_PROGRAM.md`.

---

## 🛠️ Менеджер проектов для разработчиков

WebGate содержит кроссплатформенный скрипт управления проектом для проверки окружения, контролируемой установки недостающих инструментов, компиляции и CI-верификации.

**Windows:**
```powershell
.\scripts\webgate.ps1
```

**Linux / macOS:**
```sh
./scripts/webgate.sh
```

Запуск любого лаунчера без аргументов открывает интерактивное меню. Примеры вызова команд:

```sh
python3 scripts/project_manager.py doctor
python3 scripts/project_manager.py install --dry-run
python3 scripts/project_manager.py install --yes
python3 scripts/project_manager.py verify
python3 scripts/project_manager.py build
python3 scripts/project_manager.py build --release
python3 scripts/project_manager.py package
```

Установщик строго ограничен разрешенным списком (allowlist): он не принимает произвольные имена пакетов или shell-команды и не должен оперировать конфиденциальными данными WebGate. Контракт описан в [`docs/development/PROJECT_MANAGER.md`](docs/development/PROJECT_MANAGER.md).

`verify`, успешная сборка или создание package сами по себе **не являются доказательством production qualification**. Production status определяется только закрытием обязательных security/resilience gates и release-binary qualification.

---

## 🎯 Цели проекта

1. **Доступ в один клик** к приватной документации и сервисам для доверенного пула пользователей.
2. **Отсутствие требования статического белого IP** на исходном сервере приложений.
3. **Отсутствие общесистемного VPN** в штатном режиме: трафик сторонних приложений не перехватывается.
4. **Fail-Closed:** отказ transport/authorization/runtime не может приводить к прямому публичному выходу.
5. **Множественные независимые маршруты/транспорты** с доказуемой независимостью failure domains.
6. **Конфигурация и отзыв доступа** на уровне отдельных пользователей и устройств.
7. **Криптографически проверяемые конфигурации и релизы** с anti-rollback.
8. **Интеграция с [`Homiakus/SecureAcces`](https://github.com/Homiakus/SecureAcces)** как авторитетным центром авторизации и аутентификации.
9. **Capability isolation и bounded resources** как фундамент runtime/data-plane архитектуры.
10. **WebGate-owned protected renderer/runtime** без неявного system-browser fallback.
11. **Android и Windows как приоритетные production-платформы**, с дальнейшей унификацией Linux/macOS.

---

## 🏛️ Каноническая архитектура

* **Protected browser/runtime:** текущая Servo-ветка остается measured compatibility/qualification lane; в `MASTER_PLAN.md` развивается WebGate Browser Runtime (WGBR) с zero-network renderer, capability IPC и ограниченным WGWeb profile. Никакой renderer не может считаться `Open` без положительного runtime evidence.
* **Общее ядро безопасности:** Rust для security policy, device identity, broker boundaries и transport-side contracts; дальнейшее уменьшение числа независимых state machine является отдельной целью архитектуры.
* **Изоляция сети:** browser/runtime работает через destination-restricted loopback boundary; системный TUN в штатном режиме не требуется.
* **Transport supervisor:** отказоустойчивость должна управляться одним authoritative state machine с hysteresis, bounded recovery и evidence-based health.
* **Relay plane:** целевая production-модель требует per-node identity, authenticated encrypted envelope, explicit routing и admission control. Shared cluster-secret / ambiguous routing не считаются конечной production-моделью.
* **Идентификация устройств:** аппаратно защищенные ключи там, где платформа это позволяет, плюс криптографически проверяемая device identity.
* **Серверный шлюз:** Go data/control plane с server-owned routing, SecureAcces authorization boundary и durable registries.
* **Resource Governor:** количество соединений, streams, queues, workers, памяти и bandwidth должно быть ограничено явными квотами.
* **Release Trust Kernel:** production update может быть принят только после локального вычисления digest, реальной signature verification, platform check и anti-rollback проверки.

---

## 🧠 Математическая модель устойчивости

WebGate рассматривается как управляемая распределенная система:

```text
state X(t)
  ├── routing
  ├── queues/resources
  ├── latency/errors
  ├── liveness
  ├── authorization
  ├── policy epoch
  └── release/config epoch
        ↓
controller U(t)
  ├── admit
  ├── route
  ├── failover
  ├── shed
  ├── recover
  └── revoke
```

Базовый принцип:

```text
Capability isolation
+ Bounded resources
+ Evidence-based state
+ Explicit routing
+ Formal invariants
```

Подробная модель и найденные failure loops описаны в [`docs/research/CYBERNETIC_STABILITY_AUDIT_2026-09-08.md`](docs/research/CYBERNETIC_STABILITY_AUDIT_2026-09-08.md).

План реализации: [`docs/implementation/CYBERNETIC_STABILITY_PROGRAM.md`](docs/implementation/CYBERNETIC_STABILITY_PROGRAM.md).

---

## 📱 Платформенные уровни

Точный tier/status определяется `MASTER_PLAN.md` и qualification evidence. Историческая матрица платформ не должна интерпретироваться как доказательство равной production-зрелости.

Целевые направления:

```text
Tier 1 target  Windows x86_64
Tier 1 target  Android arm64
Tier 2 target  Linux x86_64/aarch64
Tier 2 target  macOS arm64/x86_64
Research       OpenHarmony / дополнительные платформы
```

---

## 🔒 Непреложные правила безопасности

1. **Fail-Closed:** отказ transport/authority/runtime не создаёт direct fallback.
2. **Application-scoped routing:** штатный режим не меняет OS default route.
3. **Renderer proof:** пользовательское состояние `Open` возможно только после положительного доказательства реального renderer/runtime пути.
4. **Один ключ на устройство / per-node identity:** закрытые ключи генерируются и хранятся локально; production relay/origin identity должна быть уникальной и ротируемой.
5. **Release verification:** наличие непустой signature-строки не считается подписью; production verifier обязан реально проверить digest и криптографическую signature над canonical manifest.
6. **SecureAcces authority:** transport reachability не является authorization.
7. **Explicit routing:** transit stream не может выбирать «первый доступный» Origin.
8. **Bounded resources:** нет неограниченных thread/task/stream/queue allocations.
9. **No global HOL failure:** один медленный stream не должен блокировать независимые streams.
10. **Unknown = deny:** неизвестные методы, маршруты, ключи, версии и policy states закрываются по умолчанию.
11. **Truthful readiness:** compile/test/package success не равен `ProductionQualified`.

---

## 🚨 Обязательные P0 перед production qualification

Актуальная детальная декомпозиция находится в `MASTER_PLAN.md` и Cybernetic Stability Program. После аудита 2026-09-08 к production blockers явно относятся:

- реальная cryptographic release verification;
- native secure relay envelope и per-node identity;
- explicit multi-origin routing;
- admission/resource governor;
- устранение connection-wide head-of-line failure semantics;
- единый authoritative failover/recovery supervisor;
- release-binary chaos/load/soak qualification;
- отсутствие противоречий между заявленным статусом и реальным evidence.

Формальный gate:

```text
ProductionQualified =
    no_open_P0
    AND release_trust_qualified
    AND relay_trust_qualified
    AND routing_qualified
    AND admission_qualified
    AND failover_qualified
    AND release_binary_E2E
    AND soak_pass
    AND chaos_pass
```

---

## 📁 Структура репозитория

```text
WebGate/
├── README.md
├── MASTER_PLAN.md                      # Единственный владелец canonical task status
├── Cargo.toml
├── crates/
│   ├── webgate-core/
│   ├── webgate-transport/
│   ├── webgate-browser/
│   ├── webgate-platform/
│   └── webgate-app/
├── server/
│   ├── cmd/
│   └── pkg/
├── examples/
├── scripts/
└── docs/
    ├── architecture/
    ├── development/
    ├── implementation/
    │   └── CYBERNETIC_STABILITY_PROGRAM.md
    ├── integration/
    └── research/
        └── CYBERNETIC_STABILITY_AUDIT_2026-09-08.md
```

---

## 🚦 Текущий статус

**ACTIVE SECURITY / RESILIENCE CONVERGENCE.**

Реализован значительный фундамент WebGate, включая fail-closed политики, durable state, gateway boundaries и экспериментальные/квалифицированные части transport stack. Однако текущий `main` **не должен маркироваться как production-qualified**, пока открыты обязательные P0 security/resilience tasks.

Источники истины:

1. `MASTER_PLAN.md` — canonical task/status owner.
2. `docs/implementation/CYBERNETIC_STABILITY_PROGRAM.md` — детальный execution contract новой stability tranche.
3. `docs/research/CYBERNETIC_STABILITY_AUDIT_2026-09-08.md` — математическое обоснование рисков и архитектурных изменений.
