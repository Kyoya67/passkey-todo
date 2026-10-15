const apiInput = document.querySelector('#api-url');
const list = document.querySelector('#todo-list');
const message = document.querySelector('#message');
const searchTitle = document.querySelector('#search-title');
const authDialog = document.querySelector('#auth-dialog');
const todoApp = document.querySelector('#todo-app');
const authStatus = document.querySelector('#auth-status');
const passkeyButton = document.querySelector('#register-passkey');
const loginButton = document.querySelector('#login-passkey');
const themeToggles = document.querySelectorAll('[data-theme-toggle]');
let theme = 'light';
try {
  theme =
    window.localStorage?.getItem('todo-theme') ||
    (window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
} catch {
  /* Storage may be disabled. */
}
const setTheme = (nextTheme) => {
  theme = nextTheme;
  document.documentElement.dataset.theme = theme;
  for (const toggle of themeToggles) {
    const label = theme === 'dark' ? 'ライトモードに切り替える' : 'ダークモードに切り替える';
    toggle.setAttribute('aria-label', label);
    toggle.setAttribute('title', label);
    toggle.setAttribute('aria-pressed', String(theme === 'dark'));
  }
  try {
    window.localStorage?.setItem('todo-theme', theme);
  } catch {
    /* Storage may be disabled. */
  }
};
setTheme(theme);
for (const toggle of themeToggles) {
  toggle.addEventListener('click', () => setTheme(theme === 'dark' ? 'light' : 'dark'));
}
const defaultAPIURL = ['localhost', '127.0.0.1', '[::1]'].includes(window.location.hostname)
  ? 'http://localhost:8080'
  : 'https://api.ca-techlounge-todo.kyo8.dev';
apiInput.textContent = defaultAPIURL;
const apiURL = () => {
  return defaultAPIURL;
};
const request = async (path, options = {}) => {
  const headers = options.body ? { 'Content-Type': 'application/json' } : {};
  const response = await fetch(`${apiURL()}${path}`, {
    ...options,
    credentials: 'include',
    headers: { ...headers, ...options.headers },
  });
  if (!response.ok) {
    const error = new Error(`HTTP ${response.status}`);
    error.status = response.status;
    throw error;
  }
  return response.status === 204 ? null : response.json();
};
const show = (text, error = false) => {
  message.textContent = text;
  message.style.color = error ? 'var(--status-error)' : 'var(--status-success)';
};
const showAuthStatus = (text, error = false) => {
  authStatus.textContent = text;
  authStatus.style.color = error ? 'var(--status-error)' : 'var(--status-success)';
};
const showAuth = (text = '') => {
  todoApp.hidden = true;
  list.replaceChildren();
  showAuthStatus(text, Boolean(text));
  if (!authDialog.open) authDialog.showModal();
};
const showTodoApp = () => {
  todoApp.hidden = false;
  if (authDialog.open) authDialog.close();
};
// A session is required; Escape must not reveal the TODO controls.
authDialog.addEventListener('cancel', (event) => event.preventDefault());
const fromBase64URL = (value) => {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/');
  const binary = atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, '='));
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
};
const toBase64URL = (value) => {
  const bytes = new Uint8Array(value);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
};
const setAuthBusy = (busy) => {
  passkeyButton.disabled = busy;
  loginButton.disabled = busy;
};
const login = async (afterRegistration = false) => {
  if (!window.PublicKeyCredential || !navigator.credentials?.get) {
    showAuthStatus('このブラウザではパスキーを利用できません。', true);
    return;
  }
  setAuthBusy(true);
  showAuthStatus(
    afterRegistration ? '登録できました。続けてログインしています…' : 'ログインを開始しています…'
  );
  try {
    const { loginId, options } = await request('/auth/login/begin', { method: 'POST' });
    const publicKey = options.publicKey;
    const credential = await navigator.credentials.get({
      publicKey: {
        ...publicKey,
        challenge: fromBase64URL(publicKey.challenge),
        allowCredentials: publicKey.allowCredentials?.map((item) => ({
          ...item,
          id: fromBase64URL(item.id),
        })),
      },
    });
    if (!credential) throw new Error('パスキーを取得できませんでした');
    await request('/auth/login/finish', {
      method: 'POST',
      body: JSON.stringify({
        loginId,
        credential: {
          id: credential.id,
          rawId: toBase64URL(credential.rawId),
          type: credential.type,
          response: {
            clientDataJSON: toBase64URL(credential.response.clientDataJSON),
            authenticatorData: toBase64URL(credential.response.authenticatorData),
            signature: toBase64URL(credential.response.signature),
            userHandle: credential.response.userHandle
              ? toBase64URL(credential.response.userHandle)
              : '',
          },
          clientExtensionResults: credential.getClientExtensionResults(),
          authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
        },
      }),
    });
    if (await load()) {
      showTodoApp();
    } else if (!authStatus.textContent) {
      showAuthStatus('ログイン後のセッションを確認できませんでした。', true);
    }
  } catch (error) {
    const detail =
      error.name === 'NotAllowedError'
        ? 'ログインがキャンセルされたか、時間切れになりました。'
        : `ログインに失敗しました（${error.message}）。最初からお試しください。`;
    showAuthStatus(afterRegistration ? `登録は完了しました。${detail}` : detail, true);
  } finally {
    setAuthBusy(false);
  }
};
passkeyButton.addEventListener('click', async () => {
  if (!window.PublicKeyCredential || !navigator.credentials?.create) {
    showAuthStatus('このブラウザではパスキーを利用できません。', true);
    return;
  }
  setAuthBusy(true);
  showAuthStatus('登録を開始しています…');
  let registered = false;
  try {
    const { registrationId, options } = await request('/auth/registration/begin', {
      method: 'POST',
    });
    const publicKey = options.publicKey;
    const credential = await navigator.credentials.create({
      publicKey: {
        ...publicKey,
        challenge: fromBase64URL(publicKey.challenge),
        user: { ...publicKey.user, id: fromBase64URL(publicKey.user.id) },
        excludeCredentials: publicKey.excludeCredentials?.map((item) => ({
          ...item,
          id: fromBase64URL(item.id),
        })),
      },
    });
    if (!credential) throw new Error('パスキーを作成できませんでした');
    await request('/auth/registration/finish', {
      method: 'POST',
      body: JSON.stringify({
        registrationId,
        credential: {
          id: credential.id,
          rawId: toBase64URL(credential.rawId),
          type: credential.type,
          response: {
            clientDataJSON: toBase64URL(credential.response.clientDataJSON),
            attestationObject: toBase64URL(credential.response.attestationObject),
            transports: credential.response.getTransports?.(),
          },
          clientExtensionResults: credential.getClientExtensionResults(),
          authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
        },
      }),
    });
    registered = true;
  } catch (error) {
    showAuthStatus(
      error.name === 'NotAllowedError'
        ? '登録がキャンセルされたか、時間切れになりました。もう一度お試しください。'
        : `登録に失敗しました（${error.message}）。もう一度最初からお試しください。`,
      true
    );
  } finally {
    setAuthBusy(false);
  }
  if (registered) await login(true);
});
loginButton.addEventListener('click', () => login());
const escapeHTML = (value) =>
  String(value).replace(
    /[&<>'"]/g,
    (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' })[c]
  );
const columns = [
  { status: 'not_started', label: '未着手' },
  { status: 'in_progress', label: '進行中' },
  { status: 'completed', label: '完了' },
];
const priorityLevels = [
  { value: 'high', label: '高' },
  { value: 'medium', label: '中' },
  { value: 'low', label: '小' },
];
const priorityLevel = (todo) => todo.priorityLevel || 'medium';
const sortByPriority = (a, b) => (a.priority ?? 0) - (b.priority ?? 0) || a.id - b.id;
let currentTodos = new Map();
let draggedTodoID = null;
let draggedCard = null;
let dragPlacement = null;
let statusBusy = false;
let touchDrag = null;

const animateLayout = (previousPositions) => {
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;
  for (const card of list.querySelectorAll('.todo')) {
    const previous = previousPositions.get(card.dataset.id);
    if (!previous || !card.animate) continue;
    const current = card.getBoundingClientRect();
    const x = previous.left - current.left;
    const y = previous.top - current.top;
    if (Math.abs(x) < 1 && Math.abs(y) < 1) continue;
    card.animate([{ transform: `translate(${x}px, ${y}px)` }, { transform: 'translate(0, 0)' }], {
      duration: 260,
      easing: 'cubic-bezier(0.2, 0.75, 0.25, 1)',
    });
  }
};

const renderCard = (todo, rank = null) => `
  <article class="todo" data-id="${escapeHTML(todo.id)}" draggable="true">
    <div class="todo-details">
      <div class="todo-title-row">${rank === null ? '' : `<span class="todo-rank" aria-label="グループ内順位 ${rank}">${rank}</span>`}<p class="todo-title">${escapeHTML(todo.title)}</p></div>
      <p class="todo-meta">更新：${new Date(todo.updatedAt).toLocaleString('ja-JP')}</p>
    </div>
    <div class="todo-actions">
      <button class="button secondary edit" type="button">編集</button>
      <button class="button secondary delete" type="button">削除</button>
    </div>
  </article>`;

const renderBoard = (todos) => {
  const previousPositions = new Map(
    [...list.querySelectorAll('.todo')].map((card) => [
      card.dataset.id,
      card.getBoundingClientRect(),
    ])
  );
  currentTodos = new Map(todos.map((todo) => [String(todo.id), todo]));
  list.innerHTML = columns
    .map(({ status, label }) => {
      const cards = todos.filter((todo) => todo.status === status);
      const contents =
        status === 'completed'
          ? `<div class="column-cards">${cards
              .sort((a, b) => new Date(b.updatedAt) - new Date(a.updatedAt) || b.id - a.id)
              .map((todo) => renderCard(todo))
              .join('')}</div>`
          : priorityLevels
              .map(({ value, label }) => {
                const groupCards = cards
                  .filter((todo) => priorityLevel(todo) === value)
                  .sort(sortByPriority);
                return `<section class="priority-group" data-level="${value}" aria-label="優先度 ${label}"><div class="priority-group-heading"><span class="priority-dot priority-${value}"></span><h4>優先度：${label}</h4></div><div class="column-cards">${groupCards.map((todo, index) => renderCard(todo, index + 1)).join('')}</div></section>`;
              })
              .join('');
      return `<section class="kanban-column" data-status="${status}" aria-label="${label}">
        <div class="column-heading"><h3>${label}</h3><span class="column-count">${cards.length}</span></div>
        ${contents}
      </section>`;
    })
    .join('');
  animateLayout(previousPositions);
};

const clearDrag = () => {
  draggedTodoID = null;
  draggedCard = null;
  dragPlacement = null;
  list
    .querySelectorAll(
      '.is-drop-target, .is-group-target, .is-dragging, .is-drop-before, .is-drop-after'
    )
    .forEach((element) => {
      element.classList.remove(
        'is-drop-target',
        'is-group-target',
        'is-dragging',
        'is-drop-before',
        'is-drop-after'
      );
    });
};

// Animate neighbouring cards when the dragged card changes its place in the DOM.
const reorderDuringDrag = (card, target, after) => {
  const parent = target?.parentElement;
  if (!card || !parent || card === target || !parent.insertBefore) return;
  if (card.parentElement !== parent) return; // Cross-group moves are persisted on drop.
  if (after ? target.nextElementSibling === card : card.nextElementSibling === target) return;
  const before = new Map(
    [...parent.querySelectorAll('.todo')].map((element) => [
      element,
      element.getBoundingClientRect(),
    ])
  );
  parent.insertBefore(card, after ? target.nextSibling : target);
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;
  for (const [element, previous] of before) {
    if (element === card || !element.animate) continue;
    const current = element.getBoundingClientRect();
    const dy = previous.top - current.top;
    if (Math.abs(dy) < 1) continue;
    element.getAnimations?.().forEach((animation) => animation.cancel());
    element.animate([{ transform: `translateY(${dy}px)` }, { transform: 'translateY(0)' }], {
      duration: 200,
      easing: 'cubic-bezier(0.2, 0.75, 0.25, 1)',
    });
  }
};

const highlightPosition = (target, id, clientY) => {
  const column = target?.closest('.kanban-column');
  const todo = currentTodos.get(String(id));
  const group = nearestPriorityGroup(column, target, clientY);
  let card = null;
  let after = false;
  if (
    todo &&
    column?.dataset.status === todo.status &&
    todo.status !== 'completed' &&
    group?.dataset.level === priorityLevel(todo)
  ) {
    const cards = [...(group.querySelectorAll?.('.todo') || [])].filter(
      (item) => item.dataset.id !== String(id)
    );
    if (cards.length) {
      card = cards.find((item) => {
        const rect = item.getBoundingClientRect();
        return clientY < rect.top + rect.height / 2;
      });
      if (!card) {
        card = cards.at(-1);
        after = true;
      }
    } else {
      const hovered = target?.closest('.todo');
      if (hovered && hovered.dataset.id !== String(id)) {
        card = hovered;
        const rect = card.getBoundingClientRect();
        after = clientY >= rect.top + rect.height / 2;
      }
    }
  }
  const placement = card
    ? {
        sourceID: String(id),
        status: todo.status,
        level: priorityLevel(todo),
        targetID: card.dataset.id,
        after,
      }
    : null;
  if (
    dragPlacement?.targetID === placement?.targetID &&
    dragPlacement?.after === placement?.after &&
    dragPlacement?.sourceID === placement?.sourceID
  )
    return;
  list.querySelectorAll('.is-drop-before, .is-drop-after').forEach((item) => {
    item.classList.remove('is-drop-before', 'is-drop-after');
  });
  dragPlacement = placement;
  if (card) {
    const source = draggedCard || touchDrag?.card;
    if (source && source.parentElement === card.parentElement) {
      reorderDuringDrag(source, card, after);
    } else {
      card.classList.add(after ? 'is-drop-after' : 'is-drop-before');
    }
  }
};

const highlightColumn = (column) => {
  list.querySelectorAll('.is-drop-target').forEach((target) => {
    if (target !== column) target.classList.remove('is-drop-target');
  });
  column?.classList.add('is-drop-target');
};

const highlightGroup = (group) => {
  list.querySelectorAll('.is-group-target').forEach((target) => {
    if (target !== group) target.classList.remove('is-group-target');
  });
  group?.classList.add('is-group-target');
};

const nearestPriorityGroup = (column, target, clientY) => {
  if (!column || column.dataset.status === 'completed') return null;
  const hovered = target?.closest('.priority-group');
  if (hovered) return hovered;
  if (!Number.isFinite(clientY)) return null;
  const groups = column.querySelectorAll?.('.priority-group') || [];
  let nearest = null;
  let shortestDistance = Infinity;
  for (const group of groups) {
    const { top, bottom } = group.getBoundingClientRect();
    const distance = Math.max(top - clientY, clientY - bottom, 0);
    if (distance < shortestDistance) {
      nearest = group;
      shortestDistance = distance;
    }
  }
  return nearest;
};

const clearTouchDrag = () => {
  if (!touchDrag) return;
  clearTimeout(touchDrag.timer);
  touchDrag.ghost?.remove();
  touchDrag.card.classList.remove('is-dragging');
  if (touchDrag.card.hasPointerCapture?.(touchDrag.pointerId)) {
    touchDrag.card.releasePointerCapture(touchDrag.pointerId);
  }
  highlightColumn(null);
  highlightGroup(null);
  dragPlacement = null;
  list.querySelectorAll('.is-drop-before, .is-drop-after').forEach((card) => {
    card.classList.remove('is-drop-before', 'is-drop-after');
  });
  touchDrag = null;
};

const updateStatus = async (id, status, level = null) => {
  const todo = currentTodos.get(String(id));
  if (!todo || statusBusy) return;
  const nextLevel = level || priorityLevel(todo);
  if (todo.status === status && priorityLevel(todo) === nextLevel) return;
  const previousTodos = [...currentTodos.values()];
  const nextPriority =
    Math.max(
      0,
      ...previousTodos
        .filter((item) => item.status === status && priorityLevel(item) === nextLevel)
        .map((item) => item.priority || 0)
    ) + 1;
  statusBusy = true;
  list.setAttribute('aria-busy', 'true');
  renderBoard(
    previousTodos.map((item) =>
      String(item.id) === String(id)
        ? { ...item, status, priorityLevel: nextLevel, priority: nextPriority }
        : item
    )
  );
  try {
    await request(`/todos/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ title: todo.title, status, priorityLevel: nextLevel }),
    });
    if (await load()) show('');
  } catch (error) {
    if (error.status === 401) {
      showAuth('セッションが切れました。パスキーでログインしてください。');
    } else {
      show(`更新に失敗しました（${error.message}）`, true);
      renderBoard(previousTodos);
    }
  } finally {
    statusBusy = false;
    list.removeAttribute('aria-busy');
  }
};
const movePriority = async (id, targetID, after = false) => {
  const todo = currentTodos.get(String(id));
  if (!todo || todo.status === 'completed' || statusBusy) return;
  const previousTodos = [...currentTodos.values()];
  const originalOrder = previousTodos
    .filter((item) => item.status === todo.status && priorityLevel(item) === priorityLevel(todo))
    .sort(sortByPriority)
    .map((item) => String(item.id));
  const ids = originalOrder.filter((itemID) => itemID !== String(id));
  let index = targetID ? ids.indexOf(String(targetID)) : ids.length;
  if (index < 0) return;
  if (after && targetID) index++;
  const beforeId = ids[index] ? Number(ids[index]) : null;
  if (targetID === String(id)) return;
  const nextOrder = [...ids.slice(0, index), String(id), ...ids.slice(index)];
  if (nextOrder.every((itemID, position) => itemID === originalOrder[position])) return;
  const ranks = new Map(nextOrder.map((itemID, position) => [itemID, position + 1]));
  statusBusy = true;
  list.setAttribute('aria-busy', 'true');
  renderBoard(
    previousTodos.map((item) =>
      ranks.has(String(item.id)) ? { ...item, priority: ranks.get(String(item.id)) } : item
    )
  );
  try {
    await request(`/todos/${id}/position`, {
      method: 'PUT',
      body: JSON.stringify({ beforeId }),
    });
    if (await load()) show('');
  } catch (error) {
    if (error.status === 401) showAuth('セッションが切れました。パスキーでログインしてください。');
    else {
      show(`優先順位の更新に失敗しました（${error.message}）`, true);
      renderBoard(previousTodos);
    }
  } finally {
    statusBusy = false;
    list.removeAttribute('aria-busy');
  }
};
const dropTodo = async (id, column, target, clientY, placement = null) => {
  if (!column) return;
  const todo = currentTodos.get(String(id));
  if (!todo) return;
  const level = nearestPriorityGroup(column, target, clientY)?.dataset.level || priorityLevel(todo);
  if (
    todo.status !== column.dataset.status ||
    (column.dataset.status !== 'completed' && level !== priorityLevel(todo))
  ) {
    return updateStatus(id, column.dataset.status, level);
  }
  if (todo.status === 'completed') return;
  if (
    placement?.sourceID === String(id) &&
    placement.status === todo.status &&
    placement.level === level
  ) {
    return movePriority(id, placement.targetID, placement.after);
  }
  const card = target?.closest('.todo');
  const targetID = card?.dataset.id;
  const midpoint = card?.getBoundingClientRect
    ? card.getBoundingClientRect().top + card.getBoundingClientRect().height / 2
    : 0;
  return movePriority(id, targetID, Boolean(card && clientY > midpoint));
};
async function load() {
  try {
    const title = searchTitle.value.trim();
    const query = title ? `?${new URLSearchParams({ title })}` : '';
    const data = await request(`/todos${query}`);
    const todos = data.todos || [];
    renderBoard(todos);
    return true;
  } catch (error) {
    list.replaceChildren();
    if (error.status === 401) {
      showAuth(todoApp.hidden ? '' : 'セッションが切れました。パスキーでログインしてください。');
      return false;
    }
    show(`一覧取得に失敗しました（${error.message}）`, true);
    if (authDialog.open) showAuthStatus(`一覧取得に失敗しました（${error.message}）`, true);
    return false;
  }
}
document.querySelector('#search-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  await load();
});
document.querySelector('#todo-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  const input = document.querySelector('#title');
  if (!input.value.trim()) return;
  try {
    await request('/todos', {
      method: 'POST',
      body: JSON.stringify({ title: input.value.trim() }),
    });
    input.value = '';
    if (await load()) show('');
  } catch (error) {
    if (error.status === 401)
      return showAuth('セッションが切れました。パスキーでログインしてください。');
    show(`追加に失敗しました（${error.message}）`, true);
  }
});
list.addEventListener('dragstart', (event) => {
  const item = event.target.closest('.todo');
  if (!item || event.target.closest('button, input') || statusBusy || touchDrag?.active) {
    event.preventDefault();
    return;
  }
  draggedTodoID = item.dataset.id;
  draggedCard = item;
  dragPlacement = null;
  event.dataTransfer.effectAllowed = 'move';
  event.dataTransfer.setData('text/plain', draggedTodoID);
  item.classList.add('is-dragging');
});
list.addEventListener('dragover', (event) => {
  const column = event.target.closest('.kanban-column');
  if (!column || !draggedTodoID || statusBusy) return;
  if (
    column.dataset.status === 'completed' &&
    currentTodos.get(draggedTodoID)?.status === 'completed'
  ) {
    highlightColumn(null);
    highlightPosition(null, draggedTodoID, event.clientY);
    return;
  }
  event.preventDefault();
  event.dataTransfer.dropEffect = 'move';
  highlightColumn(column);
  highlightGroup(nearestPriorityGroup(column, event.target, event.clientY));
  highlightPosition(event.target, draggedTodoID, event.clientY);
});
list.addEventListener('drop', async (event) => {
  const column = event.target.closest('.kanban-column');
  if (!column || !draggedTodoID) return;
  event.preventDefault();
  const id = draggedTodoID;
  const placement = dragPlacement;
  clearDrag();
  await dropTodo(id, column, event.target, event.clientY, placement);
});
list.addEventListener('dragend', clearDrag);
list.addEventListener('dragleave', (event) => {
  const column = event.target.closest('.kanban-column');
  if (column && !column.contains(event.relatedTarget)) column.classList.remove('is-drop-target');
});
list.addEventListener('pointerdown', (event) => {
  if (event.pointerType !== 'touch' || statusBusy || event.target.closest('button, input')) return;
  const card = event.target.closest('.todo');
  if (!card) return;
  clearTouchDrag();
  touchDrag = {
    pointerId: event.pointerId,
    id: card.dataset.id,
    card,
    startX: event.clientX,
    startY: event.clientY,
    active: false,
    ghost: null,
    timer: null,
  };
  touchDrag.timer = setTimeout(() => {
    if (!touchDrag || touchDrag.pointerId !== event.pointerId) return;
    touchDrag.active = true;
    dragPlacement = null;
    card.classList.add('is-dragging');
    const ghost = card.cloneNode(true);
    ghost.classList.add('touch-drag-preview');
    ghost.setAttribute('aria-hidden', 'true');
    ghost.style.width = `${card.getBoundingClientRect().width}px`;
    ghost.style.left = `${event.clientX}px`;
    ghost.style.top = `${event.clientY}px`;
    document.body.append(ghost);
    touchDrag.ghost = ghost;
    card.setPointerCapture?.(event.pointerId);
  }, 250);
});
window.addEventListener('pointermove', (event) => {
  if (!touchDrag || event.pointerId !== touchDrag.pointerId) return;
  if (!touchDrag.active) {
    if (Math.hypot(event.clientX - touchDrag.startX, event.clientY - touchDrag.startY) > 10) {
      clearTouchDrag();
    }
    return;
  }
  event.preventDefault();
  touchDrag.ghost.style.left = `${event.clientX}px`;
  touchDrag.ghost.style.top = `${event.clientY}px`;
  const target = document.elementFromPoint(event.clientX, event.clientY);
  const column = target?.closest('.kanban-column');
  const sameCompleted =
    column?.dataset.status === 'completed' &&
    currentTodos.get(touchDrag.id)?.status === 'completed';
  highlightColumn(sameCompleted ? null : column);
  highlightGroup(sameCompleted ? null : nearestPriorityGroup(column, target, event.clientY));
  highlightPosition(target, touchDrag.id, event.clientY);
});
window.addEventListener('pointerup', async (event) => {
  if (!touchDrag || event.pointerId !== touchDrag.pointerId) return;
  const { active, id } = touchDrag;
  const placement = dragPlacement;
  const target = active ? document.elementFromPoint(event.clientX, event.clientY) : null;
  const column = target?.closest('.kanban-column');
  clearTouchDrag();
  if (column) await dropTodo(id, column, target, event.clientY, placement);
});
window.addEventListener('pointercancel', (event) => {
  if (touchDrag?.pointerId === event.pointerId) clearTouchDrag();
});
list.addEventListener('click', async (event) => {
  const item = event.target.closest('.todo');
  if (!item) return;
  const id = item.dataset.id;
  const editButton = event.target.closest('.edit');
  try {
    if (event.target.classList.contains('delete')) {
      await request(`/todos/${id}`, { method: 'DELETE' });
      if (await load()) show('');
    } else if (editButton) {
      const titleElement = item.querySelector('.todo-title');
      const editing = titleElement.querySelector('.todo-edit-input');
      if (!editing) {
        const input = document.createElement('input');
        input.className = 'todo-edit-input';
        input.value = titleElement.textContent;
        titleElement.replaceChildren(input);
        input.focus();
        editButton.textContent = '保存';
        return;
      }
      const title = editing.value.trim();
      if (!title) return;
      await request(`/todos/${id}`, { method: 'PUT', body: JSON.stringify({ title }) });
      if (await load()) show('');
    }
  } catch (error) {
    if (error.status === 401)
      return showAuth('セッションが切れました。パスキーでログインしてください。');
    show(`操作に失敗しました（${error.message}）`, true);
  }
});
document.querySelector('#logout-button').addEventListener('click', async (event) => {
  const button = event.currentTarget;
  button.disabled = true;
  try {
    await request('/auth/logout', { method: 'POST' });
    searchTitle.value = '';
    message.textContent = '';
    showAuth();
  } catch (error) {
    show(`ログアウトに失敗しました（${error.message}）`, true);
  } finally {
    button.disabled = false;
  }
});
load().then((loaded) => {
  if (loaded) showTodoApp();
  else if (!authDialog.open) showAuth('TODOを読み込めませんでした。API接続を確認してください。');
});
