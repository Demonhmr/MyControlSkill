const KEY = 'leadership-app-state-v1';

// Сейчас — localStorage. Для реального backend замените load/save
// на fetch(`/api/state`) и оставьте сигнатуры без изменений.
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
