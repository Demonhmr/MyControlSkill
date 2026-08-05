import { useEffect, useState } from 'react';
import { ROLES } from '../data/questionnaire';
import { Card, Banner } from '../components/ui.jsx';
import SurveyForm, { emptyForm } from '../components/SurveyForm.jsx';
import { fetchSurvey, submitSurvey } from '../services/survey.js';

const ROLE_LABEL = Object.fromEntries(ROLES.map((r) => [r.value, r.label]));

/**
 * Анкета для приглашённого респондента.
 *
 * Отдельный вход в приложение: ни навигации, ни доступа к профилю
 * руководителя здесь нет и быть не должно — респондент видит только вопросы.
 * Аккаунт ему не нужен, вся аутентификация это токен из ссылки.
 */
export default function RespondentSurveyScreen({ token }) {
  const [state, setState] = useState({ status: 'loading' });
  const [form, setForm] = useState(emptyForm());
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState(null);

  useEffect(() => {
    let cancelled = false;
    fetchSurvey(token)
      .then((survey) => {
        if (!cancelled) setState({ status: 'ready', survey });
      })
      .catch((err) => {
        if (!cancelled) setState({ status: 'error', error: err });
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  const submit = async () => {
    setSending(true);
    setSendError(null);
    try {
      await submitSurvey(token, form);
      setState((s) => ({ ...s, status: 'done' }));
    } catch (err) {
      setSendError(err);
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="app viz-root">
      <Shell>
        <Body
          state={state}
          form={form}
          setForm={setForm}
          submit={submit}
          sending={sending}
          sendError={sendError}
        />
      </Shell>
    </div>
  );
}

function Shell({ children }) {
  return (
    <section>
      <h1 className="scr-title">Обратная связь 360°</h1>
      {children}
    </section>
  );
}

function Body({ state, form, setForm, submit, sending, sendError }) {
  if (state.status === 'loading') {
    return <Card>Загружаем анкету…</Card>;
  }

  if (state.status === 'error') {
    const notFound = state.error?.status === 404;
    return (
      <Banner title={notFound ? 'Ссылка недействительна' : 'Не удалось открыть анкету'}>
        {notFound
          ? 'Возможно, ссылка скопирована не целиком или приглашение отозвано. Попросите отправителя прислать новую.'
          : 'Попробуйте обновить страницу чуть позже.'}
      </Banner>
    );
  }

  if (state.status === 'done') {
    return (
      <Banner tone="ok" title="Спасибо, анкета отправлена">
        Ответы обезличены: руководитель увидит только сводные цифры, но не то, кто и как ответил.
        Эту страницу можно закрыть.
      </Banner>
    );
  }

  const { survey } = state;

  if (survey.used) {
    return (
      <Banner tone="ok" title="Анкета уже заполнена">
        По этой ссылке ответы отправлены. Одна ссылка рассчитана на одного человека и работает один раз.
      </Banner>
    );
  }

  if (survey.closed) {
    return (
      <Banner title="Сбор ответов завершён">
        Раунд оценки закрыт, новые анкеты уже не принимаются.
      </Banner>
    );
  }

  return (
    <>
      <p className="scr-sub">
        Вы оцениваете: <b>{survey.leaderName}</b>. Ваша роль:{' '}
        <b>{ROLE_LABEL[survey.role] ?? survey.role}</b>.
      </p>

      <Banner tone="ok" title="Ответы анонимны">
        Кто и как ответил, не видно никому. Усреднённые оценки увидит оцениваемый, а если он состоит
        в организации — то и её HR-служба. Расчёт вообще не начнётся, пока анкеты не заполнят как
        минимум три человека. Если какой-то пункт вы не наблюдали — выбирайте «не могу оценить», это
        честнее средней оценки наугад.
      </Banner>

      {sendError && (
        <Banner title="Анкета не отправлена">
          {sendError.status === 409
            ? 'По этой ссылке анкету уже отправили или раунд закрыт.'
            : 'Проверьте соединение и попробуйте ещё раз.'}
        </Banner>
      )}

      <SurveyForm
        form={form}
        setForm={setForm}
        onSubmit={submit}
        showRole={false}
        disabled={sending}
        submitLabel={sending ? 'Отправляем…' : 'Отправить анкету'}
      />
    </>
  );
}
