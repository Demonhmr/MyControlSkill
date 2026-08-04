import { useState } from 'react';
import { Card, Banner, Button } from '../components/ui.jsx';
import { requestLoginLink } from '../services/leaderApi.js';

// Пометки, с которыми сервер возвращает на главную при неудачном переходе
// по ссылке из письма.
const LOGIN_ERRORS = {
  expired: 'Срок действия ссылки истёк. Запросите новую — она живёт 15 минут.',
  used: 'По этой ссылке уже входили. Одна ссылка работает один раз.',
  invalid: 'Ссылка недействительна. Возможно, она скопирована не целиком.',
  'no-token': 'Ссылка неполная. Скопируйте её из письма целиком.',
  server: 'Не удалось выполнить вход. Попробуйте запросить ссылку ещё раз.',
};

function initialError() {
  const reason = new URLSearchParams(window.location.search).get('login_error');
  return reason ? (LOGIN_ERRORS[reason] ?? LOGIN_ERRORS.server) : null;
}

export default function LoginScreen() {
  const [email, setEmail] = useState('');
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState(initialError);

  const submit = async (e) => {
    e.preventDefault();
    setSending(true);
    setError(null);
    try {
      await requestLoginLink(email.trim());
      setSent(true);
    } catch (err) {
      setError(
        err.status === 400
          ? 'Проверьте адрес почты.'
          : 'Не удалось отправить ссылку. Попробуйте ещё раз.'
      );
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="app viz-root">
      <section>
        <h1 className="scr-title">Компас руководителя</h1>
        <p className="scr-sub">
          Пароля нет: введите рабочую почту, и мы пришлём ссылку для входа.
        </p>

        {error && <Banner title="Не вышло войти">{error}</Banner>}

        {sent ? (
          <Banner tone="ok" title="Ссылка отправлена">
            Проверьте почту <b>{email.trim()}</b>. Ссылка действует 15 минут и сработает один раз.
          </Banner>
        ) : (
          <Card>
            <form onSubmit={submit}>
              <div className="sec-label">Рабочая почта</div>
              <input
                className="reflect"
                style={{ minHeight: 0, height: 38 }}
                type="email"
                value={email}
                placeholder="name@company.ru"
                onChange={(e) => setEmail(e.target.value)}
                aria-label="Рабочая почта"
              />
              <div className="btn-row">
                <Button type="submit" disabled={sending || email.trim() === ''}>
                  {sending ? 'Отправляем…' : 'Прислать ссылку'}
                </Button>
              </div>
            </form>
          </Card>
        )}
      </section>
    </div>
  );
}
