import { useState } from 'react';
import { useStore } from '../state/store.jsx';
import { MIN_RESPONDENTS } from '../logic/scoring';
import { Banner } from '../components/ui.jsx';
import SurveyForm, { emptyForm } from '../components/SurveyForm.jsx';

export default function Survey360Screen() {
  const { state, dispatch } = useStore();
  const [form, setForm] = useState(emptyForm());
  const [submitted, setSubmitted] = useState(false);

  const submit = () => {
    if (!form.role || !form.tenure) return;
    dispatch({ type: 'ADD_RESPONSE', response: { ...form, id: Date.now() } });
    setForm(emptyForm());
    setSubmitted(true);
    setTimeout(() => setSubmitted(false), 3000);
  };

  const externalCount = state.responses.filter((r) => r.role !== 'self').length;

  return (
    <section>
      <h1 className="scr-title">Опрос 360°</h1>
      <p className="scr-sub">
        Каждая карточка ниже — один заполненный опросник (один респондент). Собрано внешних ответов:{' '}
        <b>{externalCount}</b> из {MIN_RESPONDENTS} минимально необходимых для расчёта перцентиля.
      </p>

      {externalCount < MIN_RESPONDENTS && (
        <Banner title="Недостаточно данных">
          Пока внешних респондентов меньше {MIN_RESPONDENTS}, остальные экраны показывают демонстрационные
          значения, а не реальный расчёт — это намеренная защита от отчёта на шуме.
        </Banner>
      )}
      {submitted && <Banner tone="ok" title="Ответ сохранён">Спасибо — анкета учтена в профиле.</Banner>}

      <SurveyForm form={form} setForm={setForm} onSubmit={submit} showRole />
    </section>
  );
}
