(() => {
    'use strict';

    const PATCH_MARKER = 'WEBGATE_TRUTH_PATCH_ACTIVE';
    window.__WEBGATE_TRUTH_PATCH__ = PATCH_MARKER;
    document.documentElement.lang = 'ru';

    let coreOnline = false;
    let transportStatus = 'unknown';
    let lastBoundProfileLabel = document.getElementById('profile-tag')?.innerText || 'ПРОФИЛЬ НЕ ПОДТВЕРЖДЁН';

    function setLaunchBusy(busy, label) {
        const button = document.getElementById('btn-launch');
        const text = document.getElementById('btn-launch-text');
        if (!button || !text) return;
        button.classList.toggle('loading', busy);
        button.disabled = busy || !coreOnline;
        text.innerText = label || (busy ? 'ПРОВЕРКА...' : 'ПРОВЕРИТЬ ДОСТУП');
    }

    function ensureTruthBanner() {
        let banner = document.getElementById('truth-status-banner');
        if (banner) return banner;
        banner = document.createElement('div');
        banner.id = 'truth-status-banner';
        banner.setAttribute('role', 'status');
        banner.setAttribute('aria-live', 'polite');
        banner.style.cssText = 'position:sticky;top:0;z-index:2500;padding:.7rem 1rem;font-family:var(--font-mono);font-size:.78rem;font-weight:700;text-align:center;border-bottom:1px solid var(--border-strong);background:var(--warning-subtle);color:var(--warning);';
        document.body.prepend(banner);
        return banner;
    }

    function setTruthState(kind, message) {
        const banner = ensureTruthBanner();
        const dot = document.getElementById('transport-dot');
        const statusText = document.getElementById('transport-status-text');
        banner.innerText = message;

        if (kind === 'ready') {
            banner.style.background = 'var(--success-subtle)';
            banner.style.color = 'var(--success)';
            if (dot) {
                dot.classList.remove('offline', 'syncing');
                dot.classList.add('pulse');
            }
        } else if (kind === 'degraded') {
            banner.style.background = 'var(--warning-subtle)';
            banner.style.color = 'var(--warning)';
            if (dot) {
                dot.classList.remove('offline', 'pulse');
                dot.classList.add('syncing');
            }
        } else {
            banner.style.background = 'var(--danger-subtle)';
            banner.style.color = 'var(--danger)';
            if (dot) {
                dot.classList.remove('pulse', 'syncing');
                dot.classList.add('offline');
            }
        }

        if (statusText) statusText.innerText = message;
    }

    function clearUnverifiedProfile() {
        activeProfile = {
            name: 'Не подтверждён',
            version: '--',
            primaryRelay: '--',
            fallbackRelay: '--',
            destinations: []
        };
        selectedDestination = '';
        const mappings = [
            ['cfg-profile-name', 'Не подтверждён'],
            ['cfg-version', '--'],
            ['cfg-primary-relay', '--'],
            ['cfg-fallback-relay', '--'],
            ['bar-relay-addr', '--'],
            ['stat-profile-count', '00'],
            ['stat-dest-count', '00'],
            ['target-url-input', '']
        ];
        mappings.forEach(([id, value]) => {
            const el = document.getElementById(id);
            if (!el) return;
            if ('value' in el) el.value = value;
            else el.innerText = value;
        });
        const tag = document.getElementById('profile-tag');
        if (tag) tag.innerText = 'ПРОФИЛЬ НЕ ПОДТВЕРЖДЁН';
        renderDestinations();
    }

    function applyProfile(data) {
        if (!data || !data.primary_relay || !Array.isArray(data.destinations)) {
            throw new Error('Ядро вернуло неполный профиль');
        }
        activeProfile = {
            name: data.profile_name || data.profile_id || 'WebGate',
            version: data.version || '--',
            primaryRelay: `${data.primary_relay.address}:${data.primary_relay.port}`,
            fallbackRelay: data.fallback_relay ? `${data.fallback_relay.address}:${data.fallback_relay.port}` : '--',
            destinations: data.destinations.map(d => ({
                id: d.id,
                name: d.name,
                url: d.url,
                category: d.category || 'Общее',
                desc: d.description || ''
            }))
        };
        selectedDestination = activeProfile.destinations[0]?.url || '';
        document.getElementById('cfg-profile-name').innerText = activeProfile.name;
        document.getElementById('cfg-version').innerText = activeProfile.version;
        document.getElementById('cfg-primary-relay').innerText = activeProfile.primaryRelay;
        document.getElementById('cfg-fallback-relay').innerText = activeProfile.fallbackRelay;
        document.getElementById('bar-relay-addr').innerText = activeProfile.primaryRelay;
        document.getElementById('stat-profile-count').innerText = '01';
        document.getElementById('target-url-input').value = selectedDestination;
        document.getElementById('profile-tag').innerText = `АКТИВЕН: ${activeProfile.name}`;
        lastBoundProfileLabel = document.getElementById('profile-tag').innerText;
        renderDestinations();
    }

    async function readJsonSafely(response) {
        try {
            return await response.json();
        } catch (_) {
            return null;
        }
    }

    async function truthfulLoadLiveBackend() {
        coreOnline = false;
        clearUnverifiedProfile();
        setLaunchBusy(false, 'ЯДРО НЕДОСТУПНО');
        setTruthState('offline', 'ЯДРО WEBGATE: ПРОВЕРКА...');

        try {
            const [profileResponse, statusResponse] = await Promise.all([
                fetch('/api/profile', { cache: 'no-store' }),
                fetch('/api/status', { cache: 'no-store' })
            ]);

            if (!profileResponse.ok || !statusResponse.ok) {
                throw new Error(`core status ${profileResponse.status}/${statusResponse.status}`);
            }

            const profile = await readJsonSafely(profileResponse);
            const status = await readJsonSafely(statusResponse);
            if (!profile || !status) throw new Error('Некорректный ответ ядра');

            applyProfile(profile);
            coreOnline = true;
            transportStatus = String(status.status || 'unknown').toLowerCase();

            if ((transportStatus === 'ready' || transportStatus === 'degraded') && status.protected_proxy) {
                setTruthState(transportStatus === 'ready' ? 'ready' : 'degraded',
                    transportStatus === 'ready'
                        ? 'ЯДРО WEBGATE: ГОТОВО · ЗАЩИЩЁННЫЙ МАРШРУТ ДОСТУПЕН'
                        : 'ЯДРО WEBGATE: РАБОТАЕТ В ДЕГРАДИРОВАННОМ РЕЖИМЕ');
                setLaunchBusy(false, 'ПРОВЕРИТЬ ДОСТУП');
            } else {
                setTruthState('offline', 'ЗАЩИЩЁННЫЙ МАРШРУТ НЕДОСТУПЕН · ПРЯМОЕ ПОДКЛЮЧЕНИЕ ЗАПРЕЩЕНО');
                setLaunchBusy(false, 'МАРШРУТ НЕДОСТУПЕН');
            }
            logMessage('ЯДРО', `Подтверждён live-профиль '${activeProfile.name}', транспорт: ${transportStatus}.`, transportStatus === 'ready' ? 'ok' : 'warn');
        } catch (error) {
            coreOnline = false;
            transportStatus = 'offline';
            clearUnverifiedProfile();
            setTruthState('offline', 'ЯДРО WEBGATE НЕДОСТУПНО · ДЕМО-ДАННЫЕ НЕ ИСПОЛЬЗУЮТСЯ');
            setLaunchBusy(false, 'ЯДРО НЕДОСТУПНО');
            logMessage('ЯДРО', `Live-состояние не подтверждено: ${error.message || error}`, 'err');
        }
    }

    function navigationFailureMessage(responseStatus, data) {
        if (responseStatus === 403) return 'Доступ к приложению запрещён политикой WebGate.';
        if (responseStatus === 503 || data?.transport_status === 'offline') {
            return 'Не удалось установить защищённый маршрут. Прямое подключение заблокировано.';
        }
        if (responseStatus === 400) return 'Запрос приложения некорректен или не поддерживается.';
        return data?.message || 'Ядро WebGate не подтвердило защищённый маршрут.';
    }

    window.launchNavigation = async function launchNavigationTruthfully() {
        const url = document.getElementById('target-url-input').value.trim();
        if (!coreOnline) {
            showToast('Ядро WebGate недоступно. Защищённый запуск невозможен.', 'danger', 5000);
            setTruthState('offline', 'ЯДРО WEBGATE НЕДОСТУПНО · ЗАПУСК ЗАПРЕЩЁН');
            return;
        }
        if (!url.startsWith('webgate://') && !url.startsWith('https://')) {
            showToast('Разрешены только webgate:// и https:// назначения.', 'danger', 4500);
            logMessage('ОШИБКА_ПОЛИТИКИ', `Отклонён неподдерживаемый URL: ${url}`, 'err');
            return;
        }

        setLaunchBusy(true, 'ПРОВЕРКА...');
        logMessage('ДОСТУП', `Запрошена authoritative-проверка защищённого маршрута: ${url}`, 'normal');

        try {
            const response = await fetch('/api/navigate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ target_url: url })
            });
            const data = await readJsonSafely(response);

            if (!response.ok || !data || data.ok !== true || data.state !== 'transport_ready' || !data.protected_proxy) {
                const message = navigationFailureMessage(response.status, data);
                logMessage('ДОСТУП_ОТКЛОНЁН', message, 'err');
                showToast(message, 'danger', 5500);
                setTruthState(response.status === 403 ? 'degraded' : 'offline', message.toUpperCase());
                return;
            }

            transportStatus = data.transport_status || transportStatus;
            const message = 'Защищённый маршрут подтверждён. Реальный запуск браузера будет подтверждаться отдельно.';
            logMessage('МАРШРУТ', `${message} Цель: ${data.target}`, 'ok');
            showToast(message, 'success', 4500);
            setTruthState(transportStatus === 'degraded' ? 'degraded' : 'ready',
                transportStatus === 'degraded'
                    ? 'ЗАЩИЩЁННЫЙ МАРШРУТ ПОДТВЕРЖДЁН · РЕЖИМ DEGRADED'
                    : 'ЗАЩИЩЁННЫЙ МАРШРУТ ПОДТВЕРЖДЁН');
        } catch (error) {
            coreOnline = false;
            const message = 'Связь с ядром WebGate потеряна. Защищённый запуск не подтверждён.';
            logMessage('ЯДРО', `${message} ${error.message || error}`, 'err');
            showToast(message, 'danger', 5500);
            setTruthState('offline', 'ЯДРО WEBGATE НЕДОСТУПНО · ЗАЩИЩЁННЫЙ ЗАПУСК НЕ ПОДТВЕРЖДЁН');
        } finally {
            setLaunchBusy(false, coreOnline ? 'ПРОВЕРИТЬ ДОСТУП' : 'ЯДРО НЕДОСТУПНО');
        }
    };

    function installTruthfulConfigBinder() {
        const legacyInput = document.getElementById('file-input');
        if (!legacyInput || !legacyInput.parentNode) return;
        const input = legacyInput.cloneNode(true);
        legacyInput.parentNode.replaceChild(input, legacyInput);

        input.addEventListener('change', function(event) {
            const file = event.target.files?.[0];
            if (!file) return;
            const reader = new FileReader();
            reader.onload = async function(loadEvent) {
                const content = loadEvent.target.result;
                const profileTag = document.getElementById('profile-tag');
                const previousLabel = lastBoundProfileLabel;
                profileTag.innerText = `ПРОВЕРКА: ${file.name}`;
                showToast(`Проверка конфигурации ${file.name}...`, 'info', 2500);

                try {
                    const response = await fetch('/api/bind_config', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ filename: file.name, content })
                    });
                    const data = await readJsonSafely(response);
                    if (!response.ok || !data || data.status !== 'ok') {
                        throw new Error(data?.message || `HTTP ${response.status}`);
                    }

                    await truthfulLoadLiveBackend();
                    if (!coreOnline) throw new Error('После записи ядро не подтвердило новый профиль');
                    profileTag.innerText = `АКТИВЕН: ${file.name}`;
                    lastBoundProfileLabel = profileTag.innerText;
                    logMessage('КОНФИГ', `Ядро подтвердило профиль '${data.profile_name || data.profile_id}'.`, 'ok');
                    showToast(`Конфигурация подтверждена ядром: ${file.name}`, 'success', 4000);
                } catch (error) {
                    profileTag.innerText = previousLabel;
                    logMessage('КОНФИГ_ОТКЛОНЁН', `Профиль не изменён: ${error.message || error}`, 'err');
                    showToast(`Конфигурация отклонена. Активный профиль не изменён.`, 'danger', 5500);
                    await truthfulLoadLiveBackend();
                } finally {
                    input.value = '';
                }
            };
            reader.readAsText(file);
        });
    }

    // Remove the credibility of pre-rendered mock values before any async request finishes.
    clearUnverifiedProfile();
    setTruthState('offline', 'ЯДРО WEBGATE: LIVE-СОСТОЯНИЕ ЕЩЁ НЕ ПОДТВЕРЖДЕНО');
    setLaunchBusy(false, 'ПРОВЕРКА ЯДРА...');
    installTruthfulConfigBinder();
    truthfulLoadLiveBackend();
})();
