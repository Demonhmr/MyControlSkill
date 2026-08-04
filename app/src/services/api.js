const KEY = 'leadership-app-state-v1';

// Сейчас — localStorage (демо-режим, работает офлайн в .exe).
//
// Сетевой режим появится здесь же, но сигнатуры сохранить не выйдет: они
// станут асинхронными, а вместо одного блоба будут раздельные вызовы.
// Причина — сырые ответы 360° нельзя отдавать клиенту вообще: профиль
// считает сервер, клиент получает только агрегат. См. README, «Дальнейшие
// шаги», и cmd/server.
export function loadState() {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function saveState(partialState) {
  try {
    localStorage.setItem(KEY, JSON.stringify(partialState));
  } catch {
    // тихо игнорируем (например, приватный режим браузера)
  }
}

export function clearState() {
  localStorage.removeItem(KEY);
}
