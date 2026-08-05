import { useCallback, useEffect, useState } from 'react';
import { Card, Banner, Button, Badge } from './ui.jsx';
import { fetchMembership, setProfileConsent } from '../services/leaderApi.js';

function formatDate(iso) {
  if (!iso) return '';
  return new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' });
}

/**
 * Согласие на показ своего профиля HR-службе.
 *
 * Решение принимает только сам руководитель. Эйчар может добавить его в
 * состав организации, но разрешить показывать свои числа за него не может:
 * обратная связь, которую видит работодатель, и обратная связь для себя —
 * разные вещи, и человек должен понимать, на что соглашается.
 *
 * Ничего не показывает, если руководитель не состоит в организации.
 */
export default function ConsentCard() {
  const [state, setState] = useState({ status: 'loading' });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const load = useCallback(async () => {
    try {
      setState({ status: 'ready', membership: await fetchMembership() });
    } catch (err) {
      // 404 — организации нет, и это нормальное состояние, а не сбой.
      setState({ status: err.status === 404 ? 'no-org' : 'error' });
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (state.status !== 'ready') return null;

  const { membership } = state;
  const granted = membership.consentGranted;

  const toggle = async () => {
    setBusy(true);
    setError(null);
    try {
      await setProfileConsent(!granted);
      await load();
    } catch {
      setError('Не удалось сохранить решение. Попробуйте ещё раз.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <h3>Организация: {membership.org?.name}</h3>
      {error && <Banner title="Не вышло">{error}</Banner>}

      <div className="leader-row">
        <div className="lname">Показывать мой профиль HR-службе</div>
        <div className="chipset">
          <Badge tone={granted ? 'ok' : 'neutral'}>{granted ? 'разрешено' : 'не разрешено'}</Badge>
        </div>
      </div>

      <p className="footnote">
        {granted ? (
          <>
            HR-служба видит перцентили ваших деструкторов, две сильные стороны и признак критической
            зоны — но не отдельные ответы и не то, кто их дал. Разрешение дано{' '}
            {formatDate(membership.consentAt)}; его можно отозвать в любой момент, и числа пропадут
            из сводки сразу.
          </>
        ) : (
          <>
            Сейчас HR-служба видит вас в составе организации, но не видит ни чисел, ни того, сколько
            анкет вы собрали. Разрешение можно дать и отозвать в любой момент.
          </>
        )}
      </p>

      <div className="btn-row">
        <Button variant={granted ? 'ghost' : undefined} onClick={toggle} disabled={busy}>
          {granted ? 'Отозвать разрешение' : 'Разрешить показ HR-службе'}
        </Button>
      </div>
    </Card>
  );
}
