import { Button, Group, Modal, Stack, Text, TextInput } from "@mantine/core";
import { useState } from "react";
import type { JSX, SubmitEvent as FormSubmitEvent } from "react";

import { api, type ReportBody } from "../../api/client";

interface Props {
  report: ReportBody;
  onChanged: () => void;
}

const saveFailed = "Salvataggio non riuscito";
const deleteFailed = "Eliminazione non riuscita";
const titleRequired = "Il titolo è obbligatorio";

export function LocalReportEditor({ report, onChanged }: Props): JSX.Element {
  const [title, setTitle] = useState(report.title ?? "");
  const [date, setDate] = useState(report.date?.slice(0, 10) ?? "");
  const [provider, setProvider] = useState(report.provider);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const save = async (): Promise<void> => {
    const trimmedTitle = title.trim();
    if (trimmedTitle === "") {
      setError(titleRequired);
      return;
    }
    const trimmedProvider = provider.trim();
    setBusy(true);
    setError(undefined);
    try {
      const { response } = await api.PUT("/api/reports/local", {
        body: {
          id: report.abstractId,
          title: trimmedTitle,
          ...(date === "" ? {} : { date }),
          ...(trimmedProvider === "" ? {} : { provider: trimmedProvider }),
        },
      });
      if (!response.ok) {
        setError(saveFailed);
        return;
      }
      onChanged();
    } catch {
      setError(saveFailed);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (): Promise<void> => {
    setBusy(true);
    setError(undefined);
    try {
      const { response } = await api.DELETE("/api/reports/local", {
        body: { id: report.abstractId },
      });
      if (!response.ok) {
        setError(deleteFailed);
        setConfirming(false);
        return;
      }
      onChanged();
    } catch {
      setError(deleteFailed);
      setConfirming(false);
    } finally {
      setBusy(false);
    }
  };

  const handleSubmit = (event: FormSubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    void save();
  };

  return (
    <form onSubmit={handleSubmit} noValidate>
      <Stack gap="sm">
        <TextInput
          label="Titolo"
          value={title}
          onChange={(event) => {
            setTitle(event.currentTarget.value);
          }}
        />
        <TextInput
          label="Data"
          type="date"
          value={date}
          onChange={(event) => {
            setDate(event.currentTarget.value);
          }}
        />
        <TextInput
          label="Struttura"
          value={provider}
          onChange={(event) => {
            setProvider(event.currentTarget.value);
          }}
        />
        {error === undefined ? null : (
          <Text size="sm" c="red" role="alert">
            {error}
          </Text>
        )}
        <Group justify="space-between">
          <Button
            variant="light"
            color="red"
            disabled={busy}
            onClick={() => {
              setConfirming(true);
            }}
          >
            Elimina referto
          </Button>
          <Button type="submit" loading={busy}>
            Salva modifiche
          </Button>
        </Group>
      </Stack>
      <Modal
        opened={confirming}
        onClose={() => {
          setConfirming(false);
        }}
        title="Eliminare il referto?"
        centered
      >
        <Stack gap="sm">
          <Text>Il referto caricato verrà eliminato definitivamente.</Text>
          <Group justify="flex-end">
            <Button
              variant="default"
              onClick={() => {
                setConfirming(false);
              }}
            >
              Annulla
            </Button>
            <Button
              color="red"
              loading={busy}
              onClick={() => {
                void remove();
              }}
            >
              Elimina
            </Button>
          </Group>
        </Stack>
      </Modal>
    </form>
  );
}
