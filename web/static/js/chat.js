(() => {
  'use strict';
  const root = document.getElementById('chat');
  if (!root) return;
  const status = document.getElementById('chat-status');
  const toggle = document.getElementById('chat-toggle');
  const text = document.getElementById('chat-text');
  const send = document.getElementById('chat-send');
  const messages = document.getElementById('chat-messages');
  const error = document.getElementById('chat-error');
  const online = document.getElementById('chat-online');
  const buttons = [...document.querySelectorAll('[data-room]')];
  let socket = null;
  let timer = null;
  let room = 'general';
  let wanted = true;
  let retries = 0;
  let generation = 0;
  let lastMessageID = 0;
  function showError(message) { error.textContent = message; error.hidden = !message; }
  function state(label, connected = false) {
    status.textContent = label;
    status.classList.toggle('connected', connected);
    text.disabled = send.disabled = !connected;
    toggle.textContent = wanted ? 'Відключитися' : 'Підключитися';
    if (!connected) online.textContent = '0';
  }
  function stop() {
    generation++;
    clearTimeout(timer);
    timer = null;
    if (socket) { socket.close(1000, 'Leaving room'); socket = null; }
  }
  function connect() {
    stop();
    const current = generation;
    state(retries ? 'Перепідключення…' : 'Підключення…');
    const url = new URL('/ws/chat', window.location.href);
    url.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    url.searchParams.set('room', room);
    const ws = new WebSocket(url);
    socket = ws;
    ws.addEventListener('open', () => {
      if (current !== generation) return;
      retries = 0;
      lastMessageID = 0;
      showError('');
      state('Підключено', true);
    });
    ws.addEventListener('message', event => {
      if (current !== generation) return;
      let message;
      try { message = JSON.parse(event.data); } catch { return; }
      if (message.room !== room) return;
      online.textContent = String(message.online);
      if (message.type !== 'message') return;
      // The server echoes the message; render it only here, never on submit.
      if (message.id <= lastMessageID) return;
      lastMessageID = message.id;
      const item = document.createElement('li');
      item.className = 'chat-message';
      if (String(message.user_id) === root.dataset.userId) item.classList.add('own');
      const author = document.createElement('strong');
      author.textContent = message.author;
      const time = document.createElement('time');
      time.dateTime = message.time;
      time.textContent = new Date(message.time).toLocaleTimeString('uk-UA', {hour: '2-digit', minute: '2-digit'});
      const body = document.createElement('p');
      body.textContent = message.text;
      item.append(author, time, body);
      messages.append(item);
      while (messages.children.length > 200) messages.firstElementChild.remove();
      document.getElementById('chat-empty').hidden = true;
      item.scrollIntoView({block: 'nearest'});
    });
    ws.addEventListener('close', event => {
      if (current !== generation) return;
      socket = null;
      state('Відключено');
      if ([1007, 1008, 1009, 1013].includes(event.code)) {
        wanted = false;
        state('Відключено');
        showError('З’єднання закрито. Перевірте повідомлення або увійдіть знову.');
        return;
      }
      if (wanted && retries < 5) {
        const delay = Math.min(1000 * (2 ** retries++), 10000);
        state('Перепідключення…');
        timer = setTimeout(connect, delay);
      } else if (wanted) {
        wanted = false;
        state('Відключено');
        showError('Не вдалося підключитися. Перевірте вхід і натисніть «Підключитися».');
      }
    });
  }
  function selectRoom(id) {
    room = id;
    wanted = true;
    retries = 0;
    lastMessageID = 0;
    messages.replaceChildren();
    document.getElementById('chat-empty').hidden = false;
    for (const button of buttons) {
      const selected = button.dataset.room === room;
      button.setAttribute('aria-pressed', String(selected));
      if (selected) document.getElementById('chat-room-name').textContent = button.textContent;
    }
    connect();
  }
  buttons.forEach(button => button.addEventListener('click', () => selectRoom(button.dataset.room)));
  toggle.addEventListener('click', () => {
    wanted = !wanted;
    if (wanted) { retries = 0; connect(); } else { stop(); state('Відключено'); }
  });
  document.getElementById('chat-form').addEventListener('submit', event => {
    event.preventDefault();
    const value = text.value.trim();
    if (!value || [...value].length > 2000) { showError('Введіть від 1 до 2000 символів.'); return; }
    if (!socket || socket.readyState !== WebSocket.OPEN) { showError('Спочатку підключіться до кімнати.'); return; }
    socket.send(JSON.stringify({text: value}));
    text.value = '';
    showError('');
    text.focus();
  });
  window.addEventListener('pagehide', () => { wanted = false; stop(); });
  window.addEventListener('pageshow', event => { if (event.persisted) selectRoom(room); });
  selectRoom(room);
})();
