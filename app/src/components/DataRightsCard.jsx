import { useState } from 'react';
import { Card, Banner, Button } from './ui.jsx';
import { useSession } from '../state/session.jsx';
import { deleteAccount } from '../services/leaderApi.js';

/**
 * Управление своими данными: выгрузка и удаление аккаунта.
 *
 * Выгрузка идёт обычной ссылкой, а не через fetch: сервер отдаёт файл с
 * Content-Disposition, и браузер сам сохранит его под нужным именем.
 */
export default function DataRightsCard() {
  const { leader, refresh } = useSession();
  const [confirm, setConfirm] = useState('');
  const [asking, setAsking] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const remove = async (e) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await deleteAccount(confirm.trim());
      // Аккаунта больше нет: перечитываем сессию, приложение вернётся к входу.
      await refresh();
    } catch (err) {
      setError(
        err.status === 400
          ? 'Адрес не совпадает с вашим. Введите его точно.'
          : 'Не удалось удалить аккаунт. Попробуйте ещё раз.'
      );
      setBusy(false);
    }
  };

  return (
    <Card>
      <h3>Ваши данные</h3>
      {error && <Banner title="Не вышло">{error}</Banner>}

      <p className="footnote">
        Выгрузка содержит аккаунт, раунды, приглашения, посчитанные профили и записи тренажёра.
        Отдельных ответов респондентов в ней нет: по ним восстанавливается, кто именно как ответил.
      </p>
      <div className="btn-row">
        <a className="btn" href="/api/me/export">Скачать выгрузку</a>
      </div>

      {!asking ? (
        <div className="btn-row">
          <Button variant="ghost" onClick={() => setAsking(true)}>Удалить аккаунт</Button>
        </div>
      ) : (
        <form onSubmit={remove}>
          <Banner title="Удаление необратимо">
            Вместе с аккаунтом удалятся все ваши раунды, собранные анкеты, приглашения и записи.
            Восстановить их можно будет только из копии базы, которая делается раз в сутки.
          </Banner>
          <div className="sec-label">Для подтверждения введите свой адрес: {leader?.email}</div>
          <input
            className="reflect"
            style={{ minHeight: 0, height: 38 }}
            type="email"
            value={confirm}
            aria-label="Подтверждение адресом почты"
            onChange={(e) => setConfirm(e.target.value)}
          />
          <div className="btn-row">
            <Button type="submit" variant="ghost" disabled={busy || confirm.trim() === ''}>
              {busy ? 'Удаляем…' : 'Удалить навсегда'}
            </Button>
            <Button
              type="button"
              onClick={() => {
                setAsking(false);
                setConfirm('');
                setError(null);
              }}
            >
              Отмена
            </Button>
          </div>
        </form>
      )}
    </Card>
  );
}
