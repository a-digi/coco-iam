import { useState, useRef } from 'react';
import type { SyntheticEvent } from 'react';
import { postMultipart } from '../../../api/client';
import { useNavigate } from 'react-router-dom';
import { Submit, SubmitSmall } from '../../../Shared/Components/Button';
import { FormInput } from '../../../Shared/Components/Form';

export default function StandardImport() {
  const [name, setName] = useState('');
  const [version, setVersion] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [statusType, setStatusType] = useState<'success' | 'error' | 'info' | null>(null);
  const [loading, setLoading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const navigate = useNavigate();

  const onSubmit = async (e: SyntheticEvent) => {
    e.preventDefault();
    setStatus(null);
    setStatusType(null);
    if (!file) {
      setStatus('Bitte eine Datei auswählen.');
      setStatusType('error');
      return;
    }
    const formData = new FormData();
    formData.append('title', name);
    formData.append('version', version);
    formData.append('file', file);
    try {
      setLoading(true);
      const res = await postMultipart('xbrl/import', formData);
      const message = typeof res === 'string' ? res : (res && typeof res === 'object' && 'message' in res && typeof res.message === 'string' ? res.message : 'Upload erfolgreich');
      setStatus(message);
      setStatusType('success');
      // Nach Erfolg weiterleiten
      navigate('/standards');
    } catch (err) {
      const error = err as { message?: string };
      setStatus(error?.message || 'Upload fehlgeschlagen');
      setStatusType('error');
    } finally {
      setLoading(false);
    }
  };

  const onDropHandler = (ev: React.DragEvent<HTMLDivElement>) => {
    ev.preventDefault();
    ev.stopPropagation();
    const f = ev.dataTransfer.files?.[0];
    if (f) setFile(f);
  };

  const onDragOverHandler = (ev: React.DragEvent<HTMLDivElement>) => {
    ev.preventDefault();
    ev.stopPropagation();
  };

  const openFilePicker = () => fileInputRef.current?.click();

  return (
    <form onSubmit={onSubmit} className="max-w-2xl bg-white/80 dark:bg-surface-900">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-2xl font-semibold text-gray-900 dark:text-gray-100">Upload Formular</h2>
          <p className="text-sm text-gray-600 dark:text-gray-400">Lade eine Datei zusammen mit Name und Version hoch.</p>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FormInput
          id="title"
          label="Name"
          value={name}
          onChange={setName}
          required
          placeholder="Bezeichnung"
        />

        <FormInput
          id="version"
          label="Version"
          value={version}
          onChange={setVersion}
          required
          placeholder="z.B. 1.0.0"
        />
      </div>

      <div className="mt-6">
        <label className="block mb-2 text-sm font-medium text-gray-700 dark:text-gray-300" htmlFor="file">Datei</label>

        <div
          onDrop={onDropHandler}
          onDragOver={onDragOverHandler}
          className="flex flex-col items-center justify-center w-full p-6 border-2 border-dashed rounded-lg cursor-pointer bg-gray-50 dark:bg-gray-800/60 border-gray-300 dark:border-gray-700 hover:border-blue-500 hover:bg-blue-50/50 dark:hover:border-blue-400 dark:hover:bg-blue-900/20 transition"
          onClick={openFilePicker}
          aria-label="Datei hier ablegen oder klicken zum Auswählen"
        >
          <svg className="w-12 h-12 text-gray-400 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M7 16a4 4 0 118 0m-4-12v12" /></svg>
          <p className="mt-2 text-sm text-gray-700 dark:text-gray-300">
            {file ? (
              <span className="font-medium">Ausgewählt:</span>
            ) : (
              <span className="font-medium">Datei hier ablegen</span>
            )}
            {file ? ` ${file.name}` : ' oder klicken zum Auswählen'}
          </p>
          <input
            id="file"
            ref={fileInputRef}
            type="file"
            onChange={(e) => setFile(e.target.files?.[0] || null)}
            className="hidden"
            required
          />
        </div>
        <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">Unterstützte Formate: beliebig (abhängig vom Backend). Max. Größe gemäß Serverkonfiguration.</p>
      </div>

      <div className="mt-6 flex items-center gap-3">
        <Submit loading={loading} loadingText="Lädt…" label="Upload senden" />
        <SubmitSmall
          type="button"
          onClick={() => { setName(''); setVersion(''); setFile(null); setStatus(null); setStatusType(null); }}
        >
          Zurücksetzen
        </SubmitSmall>
      </div>

      {status && (
        <div
          className={
            `mt-6 rounded-lg p-3 text-sm border ` +
            (statusType === 'success' ? 'bg-green-50 text-green-700 border-green-200 dark:bg-green-900/20 dark:text-green-200 dark:border-green-800' :
              statusType === 'error' ? 'bg-red-50 text-red-700 border-red-200 dark:bg-red-900/20 dark:text-red-200 dark:border-red-800' :
                'bg-gray-50 text-gray-700 border-gray-200 dark:bg-gray-800/40 dark:text-gray-200 dark:border-gray-700')
          }
          role={statusType === 'error' ? 'alert' : undefined}
        >
          {status}
        </div>
      )}
    </form>
  );
}
