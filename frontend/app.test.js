const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const app = fs.readFileSync(require.resolve('./app.js'), 'utf8');

const tick = () => new Promise((resolve) => setImmediate(resolve));

function setup({ failUpdate = false } = {}) {
  const listeners = {};
  const windowListeners = {};
  const calls = [];
  let dropColumn = null;
  const timers = new Map();
  let nextTimer = 1;
  const todos = [
    { id: 1, title: 'One', status: 'not_started', priority: 1, updatedAt: '2026-10-15T00:00:00Z' },
    { id: 2, title: 'Two', status: 'in_progress', priority: 1, updatedAt: '2026-10-15T00:00:00Z' },
    { id: 3, title: 'Three', status: 'completed', priority: 1, updatedAt: '2026-10-15T00:00:00Z' },
    { id: 4, title: 'Four', status: 'not_started', priority: 2, updatedAt: '2026-10-15T00:00:00Z' },
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
    document: {
      querySelector: (selector) => elements.get(selector),
      elementFromPoint: () => ({ closest: () => dropColumn }),
      body: { append() {} },
    },
    window: {
      location: { hostname: 'localhost' },
      addEventListener(type, handler) {
        windowListeners[type] = handler;
      },
    },
    navigator: {},
    URLSearchParams,
    setTimeout: (callback) => {
      const id = nextTimer++;
      timers.set(id, callback);
      return id;
    },
    clearTimeout: (id) => timers.delete(id),
    fetch: async (url, options = {}) => {
      calls.push({ url, options });
      if (options.method === 'PUT' && url.endsWith('/position')) {
        if (failUpdate) return { ok: false, status: 503 };
        return { ok: true, status: 204 };
      }
      if (options.method === 'PUT') {
        if (failUpdate) return { ok: false, status: 503 };
        const todo = todos.find((item) => item.id === Number(url.split('/').at(-1)));
        Object.assign(todo, JSON.parse(options.body));
        return { ok: true, status: 204 };
      }
      return { ok: true, status: 200, json: async () => ({ todos }) };
    },
  };
  vm.runInNewContext(app, context);
  return {
    list,
    listeners,
    windowListeners,
    calls,
    elements,
    setDropColumn: (column) => {
      dropColumn = column;
    },
    flushTimers: () => {
      for (const callback of timers.values()) callback();
      timers.clear();
    },
  };
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
  assert.doesNotMatch(list.innerHTML, /todo-status|<select/);
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
  const item = { dataset: { id: '1' }, classList: { add() {} } };
  const column = { dataset: { status: 'completed' } };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? item : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  await listeners.drop({ target: { closest: () => column }, preventDefault() {} });
  assert.match(elements.get('#message').textContent, /状態の更新に失敗/);
  assert.match(list.innerHTML, /data-status="not_started"[\s\S]*data-id="1"/);
});

test('long-press touch drag moves a card without a status menu', async () => {
  const { list, listeners, windowListeners, calls, setDropColumn, flushTimers } = setup();
  await tick();
  const ghost = { style: {}, classList: { add() {} }, setAttribute() {}, remove() {} };
  const card = {
    dataset: { id: '1' },
    classList: { add() {}, remove() {} },
    cloneNode: () => ghost,
    getBoundingClientRect: () => ({ width: 280 }),
  };
  const column = { dataset: { status: 'in_progress' }, classList: { add() {}, remove() {} } };
  setDropColumn(column);
  listeners.pointerdown({
    pointerType: 'touch',
    pointerId: 7,
    clientX: 10,
    clientY: 20,
    target: { closest: (selector) => (selector === '.todo' ? card : null) },
  });
  flushTimers();
  await windowListeners.pointerup({ pointerId: 7, clientX: 50, clientY: 20 });
  const update = calls.find(({ options }) => options.method === 'PUT');
  assert.equal(JSON.parse(update.options.body).status, 'in_progress');
  assert.match(list.innerHTML, /data-status="in_progress"[\s\S]*data-id="1"/);
});

test('a tap does not move a card', async () => {
  const { listeners, windowListeners, calls, setDropColumn } = setup();
  await tick();
  const card = { dataset: { id: '1' }, classList: { remove() {} } };
  setDropColumn({ dataset: { status: 'completed' } });
  listeners.pointerdown({
    pointerType: 'touch',
    pointerId: 8,
    clientX: 10,
    clientY: 20,
    target: { closest: (selector) => (selector === '.todo' ? card : null) },
  });
  await windowListeners.pointerup({ pointerId: 8, clientX: 10, clientY: 20 });
  assert.equal(
    calls.some(({ options }) => options.method === 'PUT'),
    false
  );
});

test('dropping within a column updates priority before the target card', async () => {
  const { listeners, calls } = setup();
  await tick();
  const source = { dataset: { id: '4' }, classList: { add() {} } };
  const column = { dataset: { status: 'not_started' } };
  const target = {
    dataset: { id: '1' },
    getBoundingClientRect: () => ({ top: 0, height: 100 }),
  };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  await listeners.drop({
    target: { closest: (selector) => (selector === '.todo' ? target : column) },
    clientY: 20,
    preventDefault() {},
  });
  const move = calls.find(({ url }) => url.endsWith('/todos/4/position'));
  assert.deepEqual(JSON.parse(move.options.body), { beforeId: 1 });
  assert.equal(
    calls.some(({ url }) => url.endsWith('/todos/4')),
    false
  );
});

test('dropping below a card places the todo at the end of the column', async () => {
  const { listeners, calls } = setup();
  await tick();
  const source = { dataset: { id: '1' }, classList: { add() {} } };
  const column = { dataset: { status: 'not_started' } };
  const target = {
    dataset: { id: '4' },
    getBoundingClientRect: () => ({ top: 0, height: 100 }),
  };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  await listeners.drop({
    target: { closest: (selector) => (selector === '.todo' ? target : column) },
    clientY: 80,
    preventDefault() {},
  });
  const move = calls.find(({ url }) => url.endsWith('/todos/1/position'));
  assert.deepEqual(JSON.parse(move.options.body), { beforeId: null });
});
