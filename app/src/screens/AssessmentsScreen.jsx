import { useCallback, useEffect, useState } from 'react';
import { useStore } from '../state/store.jsx';
import { useProfileSource } from '../state/profile.jsx';
import { ROLES } from '../data/questionnaire';
import { Card, Banner, Button, Badge } from '../components/ui.jsx';
import ConsentCard from '../components/ConsentCard.jsx';
import DataRightsCard from '../components/DataRightsCard.jsx';
import {
  closeAssessment,
  createAssessment,
  createInvite,
  deleteAssessment,
  getAssessment,
  listAssessments,
} from '../services/leaderApi.js';

const ROLE_LABEL = Object.fromEntries(ROLES.map((r) => [r.value, r.label]));

function formatDate(iso) {
  if (!iso) return '';
  return new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' });
}

/**
 * Раунды 360° и приглашения — рабочий экран сетевого режима.
 *
 * Ссылка на анкету показывается ровно один раз, сразу после выдачи: сервер
 * хранит только её хэш и повторно показать не сможет. Поэтому она остаётся
 * на экране до перезагрузки, а не прячется после первого взгляда.
 */
export default function AssessmentsScreen() {
  const [rounds, setRounds] = useState(null);
  const [selected, setSelected] = useState(null);
  const [links, setLinks] = useState({});
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const reloadList = useCallback(async () => {
    try {
      const { assessments } = await listAssessments();
      setRounds(assessments ?? []);
    } catch {
      setError('Не удалось загрузить раунды.');
      setRounds([]);
    }
  }, []);

  useEffect(() => {
    reloadList();
  }, [reloadList]);

  const openRound = async (id) => {
    setError(null);
    try {
      setSelected(await getAssessment(id));
    } catch {
      setError('Не удалось открыть раунд.');
    }
  };

  const addRound = async () => {
    const title = prompt('Название раунда (например, «Пилот, август»)');
    if (title === null) return;
    setBusy(true);
    setError(null);
    try {
      const created = await createAssessment(title.trim());
      await reloadList();
      await openRound(created.id);
    } catch {
      setError('Не удалось создать раунд.');
    } finally {
      setBusy(false);
    }
  };

  if (rounds === null) {
    return (
      <section>
        <h1 className="scr-title">Раунды 360°</h1>
        <Card>Загружаем…</Card>
      </section>
    );
  }

  return (
    <section>
      <h1 className="scr-title">Раунды 360°</h1>
      <p className="scr-sub">
        Раунд — это один замер. Повторные раунды дают динамику: именно на них строится пульс-трекер.
      </p>

      {error && <Banner title="Ошибка">{error}</Banner>}

      <ConsentCard />

      <Card>
        <div className="btn-row">
          <Button onClick={addRound} disabled={busy}>Новый раунд</Button>
        </div>
        {rounds.length === 0 ? (
          <div className="footnote">Пока ни одного раунда.</div>
        ) : (
          rounds.map((r) => (
            <div className="leader-row" key={r.id}>
              <div className="lname">
                <button className="chip" onClick={() => openRound(r.id)}>
                  {r.title || 'Без названия'}
                </button>
              </div>
              <div className="chipset">
                <Badge tone={r.counts.ready ? 'ok' : 'neutral'}>
                  {r.counts.external} из {r.counts.required}
                </Badge>
                {r.closedAt && <span className="badgechip">закрыт</span>}
                <span className="footnote">{formatDate(r.createdAt)}</span>
              </div>
            </div>
          ))
        )}
      </Card>

      <DataRightsCard />

      {selected && (
        <RoundDetail
          data={selected}
          links={links}
          setLinks={setLinks}
          onChanged={async (id) => {
            await reloadList();
            await openRound(id);
          }}
          onRemoved={async () => {
            setSelected(null);
            await reloadList();
          }}
          setError={setError}
        />
      )}
    </section>
  );
}

function RoundDetail({ data, links, setLinks, onChanged, onRemoved, setError }) {
  const { assessment, invites } = data;
  const { dispatch } = useStore();
  const { select } = useProfileSource();
  const [role, setRole] = useState('peer');
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);

  const invite = async () => {
    setBusy(true);
    setError(null);
    try {
      const created = await createInvite(assessment.id, { role, email: email.trim() });
      setLinks((prev) => ({ ...prev, [created.invite.id]: created.link }));
      setEmail('');
      await onChanged(assessment.id);
    } catch (err) {
      setError(
        err.status === 409
          ? 'Раунд закрыт — приглашения больше не выдаются.'
          : 'Не удалось создать приглашение.'
      );
    } finally {
      setBusy(false);
    }
  };

  // Раундов у руководителя несколько, а профиль показывается по одному:
  // переключение живёт здесь, рядом со списком.
  const showProfile = async () => {
    await select(assessment.id);
    dispatch({ type: 'SET_SCREEN', screen: 'destructors' });
  };

  const remove = async () => {
    if (!confirm('Удалить раунд? Все собранные по нему анкеты пропадут безвозвратно.')) return;
    setBusy(true);
    try {
      await deleteAssessment(assessment.id);
      await onRemoved();
    } catch {
      setError('Не удалось удалить раунд.');
    } finally {
      setBusy(false);
    }
  };

  const close = async () => {
    if (!confirm('Закрыть раунд? Новые анкеты приниматься не будут.')) return;
    setBusy(true);
    try {
      await closeAssessment(assessment.id);
      await onChanged(assessment.id);
    } catch {
      setError('Не удалось закрыть раунд.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <Card>
        <h3>{assessment.title || 'Без названия'}</h3>
        <p className="footnote">
          Внешних анкет: <b>{assessment.counts.external}</b> из {assessment.counts.required} нужных.
          Самооценок: {assessment.counts.self}.
          {assessment.closedAt ? ' Раунд закрыт.' : ''}
        </p>
        {!assessment.counts.ready && !assessment.closedAt && (
          <Banner title="Профиль пока не считается">
            Расчёт начнётся, когда анкеты заполнят как минимум {assessment.counts.required} человека —
            это защита от отчёта на шуме, а не техническое ограничение.
          </Banner>
        )}
        <div className="btn-row">
          {assessment.counts.ready && (
            <Button onClick={showProfile}>Смотреть профиль</Button>
          )}
          {!assessment.closedAt && (
            <Button variant="ghost" onClick={close} disabled={busy}>Закрыть раунд</Button>
          )}
          <Button variant="ghost" onClick={remove} disabled={busy}>Удалить раунд</Button>
        </div>
      </Card>

      {!assessment.closedAt && (
        <Card>
          <h3>Пригласить респондента</h3>
          <div className="sec-label">Кем он приходится вам</div>
          <div className="chips">
            {ROLES.map((r) => (
              <button
                key={r.value}
                className={`chip ${role === r.value ? 'selected' : ''}`}
                onClick={() => setRole(r.value)}
              >
                {r.label}
              </button>
            ))}
          </div>
          <div className="sec-label">Почта (необязательно)</div>
          <input
            className="reflect"
            style={{ minHeight: 0, height: 38 }}
            type="email"
            value={email}
            placeholder="colleague@company.ru"
            aria-label="Почта респондента"
            onChange={(e) => setEmail(e.target.value)}
          />
          <div className="footnote">
            Без почты ссылку придётся передать самому — письмо отправлять некуда.
          </div>
          <div className="btn-row">
            <Button onClick={invite} disabled={busy}>Выдать ссылку</Button>
          </div>
        </Card>
      )}

      <Card>
        <h3>Приглашения</h3>
        {invites.length === 0 ? (
          <div className="footnote">Пока никого не пригласили.</div>
        ) : (
          invites.map((i) => (
            <div key={i.id} style={{ marginBottom: 12 }}>
              <div className="leader-row">
                <div className="lname">{i.email || 'без почты'}</div>
                <div className="chipset">
                  <span className="badgechip">{ROLE_LABEL[i.role] ?? i.role}</span>
                  <Badge tone={i.usedAt ? 'ok' : 'neutral'}>
                    {i.usedAt ? 'ответил' : 'ждём ответа'}
                  </Badge>
                </div>
              </div>
              {links[i.id] && (
                <div className="footnote">
                  Ссылка (показывается один раз): <code>{links[i.id]}</code>
                </div>
              )}
            </div>
          ))
        )}
      </Card>
    </>
  );
}
