const apiInput = document.querySelector('#api-url');
const list = document.querySelector('#todo-list');
const message = document.querySelector('#message');
const searchTitle = document.querySelector('#search-title');
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
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return response.status === 204 ? null : response.json();
};
const show = (text, error = false) => {
  message.textContent = text;
  message.style.color = error ? '#c34e4e' : '#16816d';
};
const passkeyButton = document.querySelector('#register-passkey');
const passkeyStatus = document.querySelector('#passkey-status');
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
passkeyButton.addEventListener('click', async () => {
  if (!window.PublicKeyCredential || !navigator.credentials?.create) {
    passkeyStatus.textContent = 'このブラウザではパスキーを利用できません。';
    return;
  }
  passkeyButton.disabled = true;
  passkeyStatus.textContent = '登録を開始しています…';
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
    passkeyStatus.textContent = '登録しました。続けてログインしてください。';
    passkeyStatus.style.color = '#16816d';
  } catch (error) {
    passkeyStatus.textContent =
      error.name === 'NotAllowedError'
        ? '登録がキャンセルされたか、時間切れになりました。もう一度お試しください。'
        : `登録に失敗しました（${error.message}）。もう一度最初からお試しください。`;
    passkeyStatus.style.color = '#c34e4e';
  } finally {
    passkeyButton.disabled = false;
  }
});
const loginButton = document.querySelector('#login-passkey');
const loginStatus = document.querySelector('#login-status');
loginButton.addEventListener('click', async () => {
  if (!window.PublicKeyCredential || !navigator.credentials?.get) {
    loginStatus.textContent = 'このブラウザではパスキーを利用できません。';
    return;
  }
  loginButton.disabled = true;
  loginStatus.textContent = 'ログインを開始しています…';
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
    loginStatus.textContent = 'ログインしました。TODOのユーザー別制御は今後対応します。';
    loginStatus.style.color = '#16816d';
  } catch (error) {
    loginStatus.textContent =
      error.name === 'NotAllowedError'
        ? 'ログインがキャンセルされたか、時間切れになりました。'
        : `ログインに失敗しました（${error.message}）。最初からお試しください。`;
    loginStatus.style.color = '#c34e4e';
  } finally {
    loginButton.disabled = false;
  }
});
const escapeHTML = (value) =>
  String(value).replace(
    /[&<>'"]/g,
    (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' })[c]
  );
async function load() {
  try {
    const title = searchTitle.value.trim();
    const query = title ? `?${new URLSearchParams({ title })}` : '';
    const data = await request(`/todos${query}`);
    const todos = data.todos || [];
    list.innerHTML = todos.length
      ? todos
          .map(
            (todo) =>
              `<article class="todo" data-id="${todo.id}"><div><p class="todo-title">${escapeHTML(todo.title)}</p><p class="todo-meta">更新：${new Date(todo.updatedAt).toLocaleString('ja-JP')}</p></div><div class="todo-actions"><button class="button secondary edit">編集</button><button class="button secondary delete">削除</button></div></article>`
          )
          .join('')
      : '<div class="empty">TODOはまだありません。</div>';
  } catch (error) {
    show(`一覧取得に失敗しました（${error.message}）`, true);
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
    show(`追加に失敗しました（${error.message}）`, true);
  }
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
    show(`操作に失敗しました（${error.message}）`, true);
  }
});
document.querySelector('#health-button').addEventListener('click', async () => {
  const status = document.querySelector('#health-status');
  status.textContent = '確認中…';
  try {
    await request('/health');
    status.textContent = '接続OK';
    status.style.color = '#16816d';
  } catch {
    status.textContent = '接続失敗';
    status.style.color = '#c34e4e';
  }
});
load();
