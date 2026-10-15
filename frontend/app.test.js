const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const app = fs.readFileSync(require.resolve('./app.js'), 'utf8');

const tick = () => new Promise((resolve) => setImmediate(resolve));

function setup({
  failUpdate = false,
  savedTheme = null,
  trackLayout = false,
  reducedMotion = false,
} = {}) {
  const listeners = {};
  const windowListeners = {};
  const calls = [];
  const animations = [];
  let dropColumn = null;
  const timers = new Map();
  const storage = new Map(savedTheme ? [['todo-theme', savedTheme]] : []);
  let nextTimer = 1;
  const todos = [
    { id: 1, title: 'One', status: 'not_started', priority: 1, updatedAt: '2026-10-15T00:00:00Z' },
    { id: 2, title: 'Two', status: 'in_progress', priority: 1, updatedAt: '2026-10-15T00:00:00Z' },
    { id: 3, title: 'Three', status: 'completed', priority: 1, updatedAt: '2026-10-15T00:00:00Z' },
    { id: 4, title: 'Four', status: 'not_started', priority: 2, updatedAt: '2026-10-15T00:00:00Z' },
    { id: 5, title: 'Five', status: 'completed', priority: 0, updatedAt: '2026-10-14T00:00:00Z' },
    {
      id: 6,
      title: 'Six',
      status: 'not_started',
      priorityLevel: 'high',
      priority: 1,
      updatedAt: '2026-10-15T00:00:00Z',
    },
    {
      id: 7,
      title: 'Seven',
      status: 'not_started',
      priorityLevel: 'low',
      priority: 1,
      updatedAt: '2026-10-15T00:00:00Z',
    },
  ];
  const element = () => ({
    value: '',
    textContent: '',
    style: {},
    hidden: false,
    setAttribute(name, value) {
      this[name] = value;
    },
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
    querySelectorAll(selector) {
      if (trackLayout && selector === '.todo') {
        return [...this.innerHTML.matchAll(/<article class="todo" data-id="(\d+)"/g)].map(
          ([, id], index) => ({
            dataset: { id },
            getBoundingClientRect: () => ({ left: 0, top: index * 100, height: 80 }),
            animate: (frames, options) => animations.push({ id, frames, options }),
          })
        );
      }
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
      '#theme-toggle',
      '#auth-theme-toggle',
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
      documentElement: { dataset: {} },
      querySelector: (selector) => elements.get(selector),
      querySelectorAll: (selector) =>
        selector === '[data-theme-toggle]'
          ? [elements.get('#theme-toggle'), elements.get('#auth-theme-toggle')]
          : [],
      elementFromPoint: () => (dropColumn?.closest ? dropColumn : { closest: () => dropColumn }),
      body: { append() {} },
    },
    window: {
      location: { hostname: 'localhost' },
      matchMedia: () => ({ matches: reducedMotion }),
      localStorage: {
        getItem: (key) => storage.get(key) ?? null,
        setItem: (key, value) => storage.set(key, value),
      },
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
        const id = Number(url.split('/').at(-2));
        const moving = todos.find((item) => item.id === id);
        const { beforeId } = JSON.parse(options.body);
        const group = todos
          .filter(
            (item) =>
              item.status === moving.status &&
              (item.priorityLevel || 'medium') === (moving.priorityLevel || 'medium') &&
              item.id !== id
          )
          .sort((a, b) => a.priority - b.priority);
        const index =
          beforeId === null ? group.length : group.findIndex((item) => item.id === beforeId);
        group.splice(index, 0, moving);
        group.forEach((item, position) => {
          item.priority = position + 1;
        });
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
    animations,
    elements,
    storage,
    documentElement: context.document.documentElement,
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
  assert.doesNotMatch(list.innerHTML, /priority-select|rank-button|priority-count|todo-status/);
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
  assert.match(elements.get('#message').textContent, /更新に失敗/);
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

test('sorting updates layout immediately and animates displaced cards', async () => {
  const { list, listeners, animations } = setup({ trackLayout: true });
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
  const drop = listeners.drop({
    target: { closest: (selector) => (selector === '.todo' ? target : column) },
    clientY: 20,
    preventDefault() {},
  });
  assert.match(list.innerHTML, /data-id="4"[\s\S]*data-id="1"/);
  assert.ok(
    animations.some(({ id, frames }) => id === '4' && frames[0].transform !== 'translate(0, 0)')
  );
  await drop;
});

test('drag preview keeps the insertion point when dropping into its opened gap', async () => {
  const { listeners, calls } = setup();
  await tick();
  const source = { dataset: { id: '4' }, classList: { add() {} } };
  const targetCard = {
    dataset: { id: '1' },
    classList: { add() {} },
    getBoundingClientRect: () => ({ top: 100, height: 80 }),
  };
  const group = {
    dataset: { level: 'medium' },
    classList: { add() {} },
    querySelectorAll: () => [targetCard, source],
  };
  const column = { dataset: { status: 'not_started' }, classList: { add() {} } };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  const gap = {
    closest: (selector) =>
      selector === '.kanban-column' ? column : selector === '.priority-group' ? group : null,
  };
  listeners.dragover({ target: gap, clientY: 110, dataTransfer: {}, preventDefault() {} });
  await listeners.drop({ target: gap, clientY: 110, preventDefault() {} });
  const move = calls.find(({ url }) => url.endsWith('/todos/4/position'));
  assert.deepEqual(JSON.parse(move.options.body), { beforeId: 1 });
});

test('a failed sort restores the previous order', async () => {
  const { list, listeners, elements } = setup({ failUpdate: true, trackLayout: true });
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
  assert.match(list.innerHTML, /data-id="1"[\s\S]*data-id="4"/);
  assert.match(elements.get('#message').textContent, /優先順位の更新に失敗/);
});

test('reduced motion disables layout animation while preserving sorting', async () => {
  const { list, listeners, animations } = setup({ trackLayout: true, reducedMotion: true });
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
  assert.match(list.innerHTML, /data-id="4"[\s\S]*data-id="1"/);
  assert.equal(animations.length, 0);
});

test('completed cards ignore priority and cannot be reordered within the completed column', async () => {
  const { list, listeners, calls } = setup();
  await tick();
  assert.match(list.innerHTML, /data-status="completed"[\s\S]*data-id="3"[\s\S]*data-id="5"/);
  const source = { dataset: { id: '5' }, classList: { add() {} } };
  const column = { dataset: { status: 'completed' } };
  const target = {
    dataset: { id: '3' },
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
  assert.equal(
    calls.some(({ url }) => url.endsWith('/todos/5/position')),
    false
  );
});

test('completed cards can still be moved to another status', async () => {
  const { listeners, calls } = setup();
  await tick();
  const source = { dataset: { id: '3' }, classList: { add() {} } };
  const column = { dataset: { status: 'not_started' } };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  await listeners.drop({ target: { closest: () => column }, preventDefault() {} });
  const update = calls.find(({ url }) => url.endsWith('/todos/3'));
  assert.equal(JSON.parse(update.options.body).status, 'not_started');
});

test('priority groups have independent rank numbers and completed cards have no controls', async () => {
  const { list } = setup();
  await tick();
  const high = list.innerHTML.match(
    /data-level="high"[\s\S]*?(?=<section class="priority-group" data-level="medium")/
  )[0];
  const medium = list.innerHTML.match(
    /data-level="medium"[\s\S]*?(?=<section class="priority-group" data-level="low")/
  )[0];
  assert.match(high, /data-id="6"[\s\S]*?グループ内順位 1/);
  assert.match(medium, /data-id="1"[\s\S]*?グループ内順位 1/);
  assert.match(medium, /data-id="4"[\s\S]*?グループ内順位 2/);
  const completed = list.innerHTML.match(/data-status="completed"[\s\S]*/)[0];
  assert.doesNotMatch(completed, /todo-rank/);
});

test('dropping a card into another priority group updates its level', async () => {
  const { listeners, calls } = setup();
  await tick();
  const source = { dataset: { id: '1' }, classList: { add() {} } };
  const column = { dataset: { status: 'not_started' } };
  const group = { dataset: { level: 'high' } };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  await listeners.drop({
    target: {
      closest: (selector) =>
        selector === '.kanban-column' ? column : selector === '.priority-group' ? group : null,
    },
    preventDefault() {},
  });
  const update = calls.find(
    ({ url, options }) => url.endsWith('/todos/1') && options.method === 'PUT'
  );
  assert.equal(JSON.parse(update.options.body).priorityLevel, 'high');
});

test('dragging into an empty priority group still updates its level', async () => {
  const { listeners, calls } = setup();
  await tick();
  const source = { dataset: { id: '2' }, classList: { add() {} } };
  const column = { dataset: { status: 'in_progress' } };
  const group = { dataset: { level: 'low' } };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  await listeners.drop({
    target: {
      closest: (selector) =>
        selector === '.kanban-column' ? column : selector === '.priority-group' ? group : null,
    },
    preventDefault() {},
  });
  const update = calls.find(({ url }) => url.endsWith('/todos/2'));
  assert.equal(JSON.parse(update.options.body).priorityLevel, 'low');
});

test('column header, group gap, and lower whitespace select the nearest priority', async (t) => {
  for (const { name, clientY, level } of [
    { name: 'header', clientY: -20, level: 'high' },
    { name: 'gap', clientY: 365, level: 'low' },
    { name: 'lower whitespace', clientY: 700, level: 'low' },
  ]) {
    await t.test(name, async () => {
      const { listeners, calls } = setup();
      await tick();
      const groups = [
        { level: 'high', top: 0, bottom: 80 },
        { level: 'medium', top: 200, bottom: 280 },
        { level: 'low', top: 400, bottom: 480 },
      ].map(({ level: groupLevel, top, bottom }) => ({
        dataset: { level: groupLevel },
        getBoundingClientRect: () => ({ top, bottom }),
      }));
      const column = {
        dataset: { status: 'not_started' },
        querySelectorAll: () => groups,
      };
      const source = { dataset: { id: '1' }, classList: { add() {} } };
      listeners.dragstart({
        target: { closest: (selector) => (selector === '.todo' ? source : null) },
        dataTransfer: { setData() {} },
        preventDefault() {},
      });
      await listeners.drop({
        target: { closest: (selector) => (selector === '.kanban-column' ? column : null) },
        clientY,
        preventDefault() {},
      });
      const update = calls.find(({ url }) => url.endsWith('/todos/1'));
      assert.equal(JSON.parse(update.options.body).priorityLevel, level);
    });
  }
});

test('drag hover highlights the priority selected by pointer position', async () => {
  const { listeners } = setup();
  await tick();
  let highlighted = false;
  const group = {
    dataset: { level: 'low' },
    classList: {
      add: (name) => {
        highlighted = name === 'is-group-target';
      },
    },
    getBoundingClientRect: () => ({ top: 400, bottom: 480 }),
  };
  const column = {
    dataset: { status: 'not_started' },
    classList: { add() {} },
    querySelectorAll: () => [group],
  };
  const source = { dataset: { id: '1' }, classList: { add() {} } };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  listeners.dragover({
    target: { closest: (selector) => (selector === '.kanban-column' ? column : null) },
    clientY: 520,
    dataTransfer: {},
    preventDefault() {},
  });
  assert.equal(highlighted, true);
});

test('touch drag uses the nearest priority group in column whitespace', async () => {
  const { listeners, windowListeners, calls, setDropColumn, flushTimers } = setup();
  await tick();
  const ghost = { style: {}, classList: { add() {} }, setAttribute() {}, remove() {} };
  const card = {
    dataset: { id: '1' },
    classList: { add() {}, remove() {} },
    cloneNode: () => ghost,
    getBoundingClientRect: () => ({ width: 280 }),
  };
  const group = {
    dataset: { level: 'low' },
    getBoundingClientRect: () => ({ top: 400, bottom: 480 }),
  };
  const column = { dataset: { status: 'not_started' }, querySelectorAll: () => [group] };
  setDropColumn({ closest: (selector) => (selector === '.kanban-column' ? column : null) });
  listeners.pointerdown({
    pointerType: 'touch',
    pointerId: 9,
    clientX: 10,
    clientY: 20,
    target: { closest: (selector) => (selector === '.todo' ? card : null) },
  });
  flushTimers();
  await windowListeners.pointerup({ pointerId: 9, clientX: 50, clientY: 520 });
  const update = calls.find(({ url }) => url.endsWith('/todos/1'));
  assert.equal(JSON.parse(update.options.body).priorityLevel, 'low');
});

test('dragging to another status and priority group updates both', async () => {
  const { listeners, calls } = setup();
  await tick();
  const source = { dataset: { id: '1' }, classList: { add() {} } };
  const column = { dataset: { status: 'in_progress' } };
  const group = { dataset: { level: 'high' } };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  await listeners.drop({
    target: {
      closest: (selector) =>
        selector === '.kanban-column' ? column : selector === '.priority-group' ? group : null,
    },
    preventDefault() {},
  });
  const update = calls.find(({ url }) => url.endsWith('/todos/1'));
  assert.deepEqual(JSON.parse(update.options.body), {
    title: 'One',
    status: 'in_progress',
    priorityLevel: 'high',
  });
});

test('successful moves do not leave status messages', async () => {
  const { listeners, calls, elements } = setup();
  await tick();
  const source = { dataset: { id: '1' }, classList: { add() {} } };
  const column = { dataset: { status: 'completed' } };
  listeners.dragstart({
    target: { closest: (selector) => (selector === '.todo' ? source : null) },
    dataTransfer: { setData() {} },
    preventDefault() {},
  });
  await listeners.drop({ target: { closest: () => column }, preventDefault() {} });
  assert.equal(
    calls.some(({ url }) => url.endsWith('/todos/1')),
    true
  );
  assert.equal(elements.get('#message').textContent, '');
});

test('theme toggles work before and after login and stay synchronized', async () => {
  const { elements, storage, documentElement } = setup({ savedTheme: 'dark' });
  assert.equal(documentElement.dataset.theme, 'dark');
  const appToggle = elements.get('#theme-toggle');
  const authToggle = elements.get('#auth-theme-toggle');
  assert.equal(appToggle['aria-label'], 'ライトモードに切り替える');
  assert.equal(authToggle['aria-label'], 'ライトモードに切り替える');
  authToggle.listeners.click();
  assert.equal(documentElement.dataset.theme, 'light');
  assert.equal(storage.get('todo-theme'), 'light');
  assert.equal(appToggle['aria-label'], 'ダークモードに切り替える');
  assert.equal(authToggle['aria-pressed'], 'false');
  appToggle.listeners.click();
  assert.equal(documentElement.dataset.theme, 'dark');
  assert.equal(authToggle['aria-pressed'], 'true');
});
