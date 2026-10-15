const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const app = fs.readFileSync(require.resolve('./app.js'), 'utf8');

const tick = () => new Promise((resolve) => setImmediate(resolve));

function setup({ failUpdate = false } = {}) {
  const listeners = {};
  const calls = [];
  const todos = [
    { id: 1, title: 'One', status: 'not_started', updatedAt: '2026-10-15T00:00:00Z' },
    { id: 2, title: 'Two', status: 'in_progress', updatedAt: '2026-10-15T00:00:00Z' },
    { id: 3, title: 'Three', status: 'completed', updatedAt: '2026-10-15T00:00:00Z' },
  ];
  const element = () => ({
    value: '',
    textContent: '',
    style: {},
    hidden: false,
    addEventListener(type, handler) {
      this.listeners ??= {};
      this.listeners[type] = handler;
    },
  });
  const list = {
    innerHTML: '',
    addEventListener(type, handler) {
      listeners[type] = handler;
    },
    querySelectorAll() {
      return [];
    },
    setAttribute() {},
    removeAttribute() {},
    replaceChildren() {
      this.innerHTML = '';
    },
  };
  const elements = new Map([
    ['#todo-list', list],
    ...[
      '#api-url',
      '#message',
      '#search-title',
      '#auth-dialog',
      '#todo-app',
      '#auth-status',
      '#register-passkey',
      '#login-passkey',
      '#search-form',
      '#todo-form',
      '#logout-button',
      '#title',
    ].map((selector) => [selector, element()]),
  ]);
  const dialog = elements.get('#auth-dialog');
  dialog.open = false;
  dialog.showModal = () => {
    dialog.open = true;
  };
  dialog.close = () => {
    dialog.open = false;
  };
  const context = {
    document: { querySelector: (selector) => elements.get(selector) },
    window: { location: { hostname: 'localhost' } },
    navigator: {},
    URLSearchParams,
    fetch: async (url, options = {}) => {
      calls.push({ url, options });
      if (options.method === 'PUT') {
        if (failUpdate) return { ok: false, status: 503 };
        const todo = todos.find((item) => item.id === Number(url.split('/').at(-1)));
        Object.assign(todo, JSON.parse(options.body));
        return { ok: true, status: 200, json: async () => todo };
      }
      return { ok: true, status: 200, json: async () => ({ todos }) };
    },
  };
  vm.runInNewContext(app, context);
  return { list, listeners, calls, elements };
}

test('renders todos in three status columns', async () => {
  const { list, elements, calls } = setup();
  await tick();
  assert.equal(elements.get('#api-url').textContent, 'http://localhost:8080');
  assert.equal(
    calls.some(({ url }) => url.endsWith('/health')),
    false
  );
  assert.equal((list.innerHTML.match(/class="kanban-column"/g) || []).length, 3);
  for (const status of ['not_started', 'in_progress', 'completed']) {
    assert.match(list.innerHTML, new RegExp(`data-status="${status}"`));
  }
  assert.match(list.innerHTML, /data-id="1" draggable="true"/);
  assert.doesNotMatch(list.innerHTML, /ここにTODOを移動/);
});

test('new todo omits status so the API defaults to not started', async () => {
  const { elements, calls } = setup();
  await tick();
  elements.get('#title').value = 'New task';
  await elements.get('#todo-form').listeners.submit({ preventDefault() {} });
  const create = calls.find(({ options }) => options.method === 'POST');
  assert.deepEqual(JSON.parse(create.options.body), { title: 'New task' });
  assert.equal(elements.has('#new-status'), false);
});

test('dropping a card sends a status update and refreshes the board', async () => {
  const { list, listeners, calls } = setup();
  await tick();
  const item = { dataset: { id: '1' }, classList: { add() {} } };
  const column = { dataset: { status: 'completed' } };
  const dataTransfer = { setData() {} };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? item : null) },
    dataTransfer,
    preventDefault() {},
  });
  await listeners.drop({ target: { closest: () => column }, preventDefault() {} });
  const update = calls.find(({ options }) => options.method === 'PUT');
  assert.equal(update.url, 'http://localhost:8080/todos/1');
  assert.equal(JSON.parse(update.options.body).status, 'completed');
  assert.match(list.innerHTML, /data-status="completed"[\s\S]*data-id="1"/);
});

test('failed status update keeps the saved status visible', async () => {
  const { list, listeners, elements } = setup({ failUpdate: true });
  await tick();
  await listeners.change({
    target: { value: 'completed', matches: () => true, closest: () => ({ dataset: { id: '1' } }) },
  });
  assert.match(elements.get('#message').textContent, /状態の更新に失敗/);
  assert.match(list.innerHTML, /data-status="not_started"[\s\S]*data-id="1"/);
});

test('status menu remains an alternative to dragging', async () => {
  const { list, listeners, calls } = setup();
  await tick();
  await listeners.change({
    target: {
      value: 'in_progress',
      matches: () => true,
      closest: () => ({ dataset: { id: '1' } }),
    },
  });
  const update = calls.find(({ options }) => options.method === 'PUT');
  assert.equal(JSON.parse(update.options.body).status, 'in_progress');
  assert.match(list.innerHTML, /data-status="in_progress"[\s\S]*data-id="1"/);
});
