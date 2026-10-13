const apiInput = document.querySelector('#api-url');
const list = document.querySelector('#todo-list');
const message = document.querySelector('#message');
const savedURL = localStorage.getItem('todo-api-url');
if (savedURL) apiInput.value = savedURL;
const apiURL = () => {
  const value = apiInput.value.trim().replace(/\/$/, '');
  localStorage.setItem('todo-api-url', value);
  return value;
};
const request = async (path, options = {}) => {
  const response = await fetch(`${apiURL()}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return response.status === 204 ? null : response.json();
};
const show = (text, error = false) => {
  message.textContent = text;
  message.style.color = error ? '#c34e4e' : '#16816d';
};
const escapeHTML = (value) =>
  String(value).replace(
    /[&<>'"]/g,
    (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' })[c]
  );
async function load() {
  try {
    const data = await request('/todos');
    const todos = data.todos || [];
    document.querySelector('#todo-count').textContent = todos.length;
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
document.querySelector('#refresh-button').addEventListener('click', load);
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
