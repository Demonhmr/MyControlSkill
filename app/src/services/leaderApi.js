// Обращения к серверу от лица руководителя.
import { ApiError, isJSON, parseResponse, request } from './http.js';

/**
 * Определяет, есть ли за приложением бэкенд, и кто вошёл.
 *
 * Режим выясняется в рантайме, а не флагом сборки: одна и та же сборка
 * вшивается в .exe (там бэкенда нет вовсе) и отдаётся сервером. Флаг
 * означал бы две разные сборки и, рано или поздно, перепутанные артефакты.
 *
 * Признак — не код ответа, а тип содержимого. Лаунчер на любой неизвестный
 * путь отдаёт оболочку приложения с кодом 200, поэтому по статусу /api/me
 * от него неотличим сервер; HTML вместо JSON отличим однозначно.
 */
export async function probeSession() {
  let response;
  try {
    response = await fetch('/api/me', { headers: { Accept: 'application/json' } });
  } catch {
    // Сети нет — считаем, что бэкенда нет тоже.
    return { mode: 'local', leader: null };
  }

  if (!isJSON(response)) {
    return { mode: 'local', leader: null };
  }
  if (response.status === 401) {
    return { mode: 'server', leader: null };
  }
  try {
    return { mode: 'server', leader: await parseResponse(response) };
  } catch (err) {
    if (err instanceof ApiError) {
      return { mode: 'server', leader: null };
    }
    throw err;
  }
}

export function requestLoginLink(email) {
  return request('/api/auth/login', { method: 'POST', body: { email } });
}

export function logout() {
  return request('/api/auth/logout', { method: 'POST' });
}

export function listAssessments() {
  return request('/api/assessments');
}

export function createAssessment(title) {
  return request('/api/assessments', { method: 'POST', body: { title } });
}

export function getAssessment(id) {
  return request(`/api/assessments/${encodeURIComponent(id)}`);
}

export function closeAssessment(id) {
  return request(`/api/assessments/${encodeURIComponent(id)}/close`, { method: 'POST' });
}

export function createInvite(id, { role, email = '' }) {
  return request(`/api/assessments/${encodeURIComponent(id)}/invites`, {
    method: 'POST',
    body: { role, email },
  });
}

/** Удаляет раунд вместе со всеми собранными по нему анкетами. */
export function deleteAssessment(id) {
  return request(`/api/assessments/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

/**
 * Удаляет аккаунт и все данные. Необратимо.
 *
 * Подтверждение адресом проверяет и сервер: одного нажатия для необратимой
 * операции мало.
 */
export function deleteAccount(confirmEmail) {
  return request('/api/me', { method: 'DELETE', body: { confirmEmail } });
}

/** Участие текущего руководителя в организации. 404 — не состоит ни в одной. */
export function fetchMembership() {
  return request('/api/me/org');
}

/** Разрешить или отозвать показ своего профиля HR-службе. */
export function setProfileConsent(granted) {
  return request('/api/me/org/consent', { method: 'PUT', body: { granted } });
}

/** Сводка по организации. 404 — организации нет, 403 — нет прав HR. */
export function fetchHROverview() {
  return request('/api/hr/overview');
}

export function createOrg(name) {
  return request('/api/hr/org', { method: 'POST', body: { name } });
}

export function addOrgMember(email) {
  return request('/api/hr/members', { method: 'POST', body: { email } });
}

/** Рабочее состояние экранов и записи из тренажёра — одним запросом. */
export function fetchLeaderState() {
  return request('/api/state');
}

export function saveLeaderState(state) {
  return request('/api/state', { method: 'PUT', body: { state } });
}

export function addReflection(code, text) {
  return request('/api/reflections', { method: 'POST', body: { code, text } });
}

/**
 * Профиль раунда.
 *
 * Ниже порога респондентов сервер отвечает 423 — это не сбой, а состояние
 * сбора, поэтому счётчики из тела отказа возвращаются как обычный результат.
 */
export async function fetchProfile(id) {
  const response = await fetch(`/api/assessments/${encodeURIComponent(id)}/profile`, {
    headers: { Accept: 'application/json' },
  });
  if (response.status === 423) {
    const body = await response.json();
    return { ready: false, counts: body.counts, profile: null };
  }
  const body = await parseResponse(response);
  return { ready: true, counts: body.counts, profile: body.profile };
}
