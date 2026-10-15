const apiInput = document.querySelector('#api-url');
const list = document.querySelector('#todo-list');
const message = document.querySelector('#message');
const searchTitle = document.querySelector('#search-title');
const authDialog = document.querySelector('#auth-dialog');
const todoApp = document.querySelector('#todo-app');
const authStatus = document.querySelector('#auth-status');
const passkeyButton = document.querySelector('#register-passkey');
const loginButton = document.querySelector('#login-passkey');
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
  message.style.color = error ? '#c34e4e' : '#16816d';
};
const showAuthStatus = (text, error = false) => {
  authStatus.textContent = text;
  authStatus.style.color = error ? '#c34e4e' : '#16816d';
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
let currentTodos = new Map();
let draggedTodoID = null;
let statusBusy = false;
let touchDrag = null;

const renderCard = (todo) => `
  <article class="todo" data-id="${escapeHTML(todo.id)}" draggable="true">
    <div class="todo-details">
      <p class="todo-title">${escapeHTML(todo.title)}</p>
      <p class="todo-meta">更新：${new Date(todo.updatedAt).toLocaleString('ja-JP')}</p>
    </div>
    <div class="todo-actions">
      <button class="button secondary edit" type="button">編集</button>
      <button class="button secondary delete" type="button">削除</button>
    </div>
  </article>`;

const renderBoard = (todos) => {
  currentTodos = new Map(todos.map((todo) => [String(todo.id), todo]));
  list.innerHTML = columns
    .map(({ status, label }) => {
      const cards = todos
        .filter((todo) => todo.status === status)
        .sort((a, b) =>
          status === 'completed'
            ? new Date(b.updatedAt) - new Date(a.updatedAt) || b.id - a.id
            : (a.priority ?? 0) - (b.priority ?? 0) || a.id - b.id
        );
      return `<section class="kanban-column" data-status="${status}" aria-label="${label}">
        <div class="column-heading"><h3>${label}</h3><span class="column-count">${cards.length}</span></div>
        <div class="column-cards">${cards.map(renderCard).join('')}</div>
      </section>`;
    })
    .join('');
};

const clearDrag = () => {
  draggedTodoID = null;
  list
    .querySelectorAll('.is-drop-target, .is-dragging, .is-drop-before, .is-drop-after')
    .forEach((element) => {
      element.classList.remove('is-drop-target', 'is-dragging', 'is-drop-before', 'is-drop-after');
    });
};

const highlightPosition = (target, id, clientY) => {
  list.querySelectorAll('.is-drop-before, .is-drop-after').forEach((card) => {
    card.classList.remove('is-drop-before', 'is-drop-after');
  });
  const card = target?.closest('.todo');
  const column = target?.closest('.kanban-column');
  if (
    !card ||
    column?.dataset.status === 'completed' ||
    card.dataset.id === String(id) ||
    column?.dataset.status !== currentTodos.get(String(id))?.status
  )
    return;
  const rect = card.getBoundingClientRect();
  card.classList.add(clientY < rect.top + rect.height / 2 ? 'is-drop-before' : 'is-drop-after');
};

const highlightColumn = (column) => {
  list.querySelectorAll('.is-drop-target').forEach((target) => {
    if (target !== column) target.classList.remove('is-drop-target');
  });
  column?.classList.add('is-drop-target');
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
  list.querySelectorAll('.is-drop-before, .is-drop-after').forEach((card) => {
    card.classList.remove('is-drop-before', 'is-drop-after');
  });
  touchDrag = null;
};

const updateStatus = async (id, status) => {
  const todo = currentTodos.get(String(id));
  if (!todo || todo.status === status || statusBusy) return;
  statusBusy = true;
  list.setAttribute('aria-busy', 'true');
  try {
    await request(`/todos/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ title: todo.title, status }),
    });
    if (await load()) show('状態を更新しました');
  } catch (error) {
    if (error.status === 401) {
      showAuth('セッションが切れました。パスキーでログインしてください。');
    } else {
      show(`状態の更新に失敗しました（${error.message}）`, true);
      renderBoard([...currentTodos.values()]);
    }
  } finally {
    statusBusy = false;
    list.removeAttribute('aria-busy');
  }
};
const movePriority = async (id, targetID, after = false) => {
  const todo = currentTodos.get(String(id));
  if (!todo || todo.status === 'completed' || statusBusy) return;
  const ids = [...currentTodos.values()]
    .filter((item) => item.status === todo.status)
    .sort((a, b) => (a.priority ?? 0) - (b.priority ?? 0) || a.id - b.id)
    .map((item) => String(item.id))
    .filter((itemID) => itemID !== String(id));
  let index = targetID ? ids.indexOf(String(targetID)) : ids.length;
  if (index < 0) return;
  if (after && targetID) index++;
  const beforeId = ids[index] ? Number(ids[index]) : null;
  if (targetID === String(id)) return;
  statusBusy = true;
  list.setAttribute('aria-busy', 'true');
  try {
    await request(`/todos/${id}/position`, {
      method: 'PUT',
      body: JSON.stringify({ beforeId }),
    });
    if (await load()) show('優先順位を更新しました');
  } catch (error) {
    if (error.status === 401) showAuth('セッションが切れました。パスキーでログインしてください。');
    else show(`優先順位の更新に失敗しました（${error.message}）`, true);
  } finally {
    statusBusy = false;
    list.removeAttribute('aria-busy');
  }
};
const dropTodo = async (id, column, target, clientY) => {
  if (!column) return;
  const todo = currentTodos.get(String(id));
  if (!todo) return;
  if (todo.status !== column.dataset.status) return updateStatus(id, column.dataset.status);
  if (todo.status === 'completed') return;
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
    show('TODOを追加しました');
    await load();
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
  event.dataTransfer.effectAllowed = 'move';
  event.dataTransfer.setData('text/plain', draggedTodoID);
  item.classList.add('is-dragging');
});
list.addEventListener('dragover', (event) => {
  const column = event.target.closest('.kanban-column');
  if (!column || !draggedTodoID || statusBusy) return;
  if (column.dataset.status === 'completed' && currentTodos.get(draggedTodoID)?.status === 'completed') {
    highlightColumn(null);
    highlightPosition(null, draggedTodoID, event.clientY);
    return;
  }
  event.preventDefault();
  event.dataTransfer.dropEffect = 'move';
  highlightColumn(column);
  highlightPosition(event.target, draggedTodoID, event.clientY);
});
list.addEventListener('drop', async (event) => {
  const column = event.target.closest('.kanban-column');
  if (!column || !draggedTodoID) return;
  event.preventDefault();
  const id = draggedTodoID;
  clearDrag();
  await dropTodo(id, column, event.target, event.clientY);
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
  const sameCompleted = column?.dataset.status === 'completed' && currentTodos.get(touchDrag.id)?.status === 'completed';
  highlightColumn(sameCompleted ? null : column);
  highlightPosition(target, touchDrag.id, event.clientY);
});
window.addEventListener('pointerup', async (event) => {
  if (!touchDrag || event.pointerId !== touchDrag.pointerId) return;
  const { active, id } = touchDrag;
  const target = active ? document.elementFromPoint(event.clientX, event.clientY) : null;
  const column = target?.closest('.kanban-column');
  clearTouchDrag();
  if (column) await dropTodo(id, column, target, event.clientY);
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
      show('TODOを削除しました');
      await load();
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
      show('TODOを更新しました');
      await load();
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
