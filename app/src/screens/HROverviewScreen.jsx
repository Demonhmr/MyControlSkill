import { useCallback, useEffect, useState } from 'react';
import { COMPETENCIES, CLUSTER_COLOR_VAR, DESTRUCTORS, nameOf } from '../data';
import { Card, Banner, Button, Badge } from '../components/ui.jsx';
import Heatmap from '../components/charts/Heatmap.jsx';
import { addOrgMember, createOrg, fetchHROverview } from '../services/leaderApi.js';

const CLUSTER_OF = Object.fromEntries(COMPETENCIES.map((c) => [c.code, c.cluster]));

/**
 * HR-сводка по организации на реальных данных.
 *
 * Порог респондентов действует и здесь: пока по руководителю мало анкет,
 * показываются счётчики, а не числа. Организационные решения по трём
 * случайным оценкам — худшее применение этой методики.
 */
export default function HROverviewScreen() {
  const [state, setState] = useState({ status: 'loading' });

  const load = useCallback(async () => {
    setState({ status: 'loading' });
    try {
      setState({ status: 'ready', data: await fetchHROverview() });
    } catch (err) {
      if (err.status === 404) setState({ status: 'no-org' });
      else if (err.status === 403) setState({ status: 'forbidden' });
      else setState({ status: 'error' });
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (state.status === 'loading') {
    return (
      <section>
        <h1 className="scr-title">HR-дашборд</h1>
        <Card>Загружаем сводку…</Card>
      </section>
    );
  }

  if (state.status === 'forbidden') {
    return (
      <section>
        <h1 className="scr-title">HR-дашборд</h1>
        <Banner title="Сводка доступна только роли HR">
          Вы состоите в организации как руководитель. Сводка — это профили ваших коллег, и открывать
          её всем участникам нельзя.
        </Banner>
      </section>
    );
  }

  if (state.status === 'error') {
    return (
      <section>
        <h1 className="scr-title">HR-дашборд</h1>
        <Banner title="Не удалось загрузить сводку">Попробуйте обновить чуть позже.</Banner>
        <Card>
          <div className="btn-row"><Button onClick={load}>Повторить</Button></div>
        </Card>
      </section>
    );
  }

  if (state.status === 'no-org') {
    return <CreateOrg onCreated={load} />;
  }

  return <Overview data={state.data} onChanged={load} />;
}

function CreateOrg({ onCreated }) {
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const submit = async (e) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await createOrg(name.trim());
      await onCreated();
    } catch (err) {
      setError(err.status === 400 ? 'Укажите название.' : 'Не удалось создать организацию.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <section>
      <h1 className="scr-title">HR-дашборд</h1>
      <p className="scr-sub">
        Сводка строится по организации: несколько руководителей, у каждого свои раунды 360°.
      </p>
      {error && <Banner title="Не вышло">{error}</Banner>}
      <Card>
        <form onSubmit={submit}>
          <div className="sec-label">Название организации</div>
          <input
            className="reflect"
            style={{ minHeight: 0, height: 38 }}
            value={name}
            placeholder="ООО «Ромашка»"
            aria-label="Название организации"
            onChange={(e) => setName(e.target.value)}
          />
          <div className="btn-row">
            <Button type="submit" disabled={busy || name.trim() === ''}>Создать организацию</Button>
          </div>
        </form>
      </Card>
    </section>
  );
}

function Overview({ data, onChanged }) {
  const leaders = data.leaders ?? [];
  const ready = leaders.filter((l) => l.ready);
  const waiting = leaders.filter((l) => !l.ready);

  return (
    <section>
      <h1 className="scr-title">HR-дашборд: {data.org?.name}</h1>
      <p className="scr-sub">
        Карта деструкторов и суперсил по организации — для приоритизации поддержки, а не для
        рейтингов. Руководители без набранных анкет показаны отдельно, без чисел.
      </p>

      <AddMember onAdded={onChanged} />

      {ready.length === 0 ? (
        <Banner title="Пока считать нечего">
          Ни по одному руководителю не набралось {leaders[0]?.counts?.required ?? 3} внешних анкет.
          Числа появятся, когда команды заполнят опросники.
        </Banner>
      ) : (
        <>
          <Card>
            <h3>Тепловая карта деструкторов</h3>
            <Heatmap
              leaders={ready.map((l) => ({ name: l.name }))}
              columns={DESTRUCTORS}
              getValue={(li, ci) => percentileOf(ready[li], DESTRUCTORS[ci].id)}
            />
            <div className="legend" style={{ marginTop: 12 }}>
              <div className="item"><span className="swatch" style={{ background: '#cde2fb' }} /> Низкий перцентиль</div>
              <div className="item"><span className="swatch" style={{ background: '#104281' }} /> Высокий перцентиль (норма)</div>
              <div className="item">
                <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--status-critical)', display: 'inline-block' }} />
                {' '}Критично (&lt; 10 перц.)
              </div>
            </div>
          </Card>

          <Card>
            <h3>Карта суперсил</h3>
            {ready.map((l) => (
              <div className="leader-row" key={l.leaderId}>
                <div className="lname">{l.name}</div>
                <div className="chipset">
                  {(l.strengths ?? []).length === 0 && (
                    <span className="footnote">нет компетенций выше 70 перцентиля</span>
                  )}
                  {(l.strengths ?? []).map((s) => (
                    <span className="badgechip" key={s.code}>
                      <span
                        className="dot"
                        style={{ background: `var(${CLUSTER_COLOR_VAR[CLUSTER_OF[s.code]] ?? '--cl-1'})` }}
                      />
                      {nameOf(s.code)} · {s.percentile}
                    </span>
                  ))}
                  {l.hasCritical && <span className="flag">⚠ есть критическая зона</span>}
                </div>
              </div>
            ))}
          </Card>
        </>
      )}

      {waiting.length > 0 && (
        <Card>
          <h3>Сбор ещё идёт</h3>
          <p className="footnote">
            По этим руководителям чисел нет намеренно: ниже порога перцентили описывают случайность.
          </p>
          {waiting.map((l) => (
            <div className="leader-row" key={l.leaderId}>
              <div className="lname">{l.name}</div>
              <div className="chipset">
                <Badge tone="neutral">
                  {l.counts?.external ?? 0} из {l.counts?.required ?? 3}
                </Badge>
                {l.role === 'hr' && <span className="badgechip">HR</span>}
              </div>
            </div>
          ))}
        </Card>
      )}
    </section>
  );
}

function AddMember({ onAdded }) {
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const submit = async (e) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await addOrgMember(email.trim());
      setEmail('');
      await onAdded();
    } catch (err) {
      setError(
        err.status === 409
          ? 'Этот человек уже состоит в другой организации.'
          : 'Не удалось добавить участника.'
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <h3>Добавить руководителя</h3>
      {error && <Banner title="Не вышло">{error}</Banner>}
      <form onSubmit={submit}>
        <input
          className="reflect"
          style={{ minHeight: 0, height: 38 }}
          type="email"
          value={email}
          placeholder="lead@company.ru"
          aria-label="Почта руководителя"
          onChange={(e) => setEmail(e.target.value)}
        />
        <div className="footnote">
          Аккаунт заведётся сразу, а человек войдёт в него по своей ссылке, когда получит её.
        </div>
        <div className="btn-row">
          <Button type="submit" disabled={busy || email.trim() === ''}>Добавить</Button>
        </div>
      </form>
    </Card>
  );
}

function percentileOf(leader, code) {
  const found = (leader.destructors ?? []).find((d) => d.code === code);
  return found ? found.percentile : 0;
}
