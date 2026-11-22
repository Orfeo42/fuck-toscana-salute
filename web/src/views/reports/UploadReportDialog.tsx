import { FileInput, Group, Modal, Stack, Text, TextInput } from "@mantine/core";
import { HardDrive, Upload, X } from "lucide-react";
import { useContext, useState } from "react";
import type { JSX, SubmitEvent as FormSubmitEvent } from "react";

import {
  api,
  type ImportDriveRequest,
  type ReportBody,
} from "../../api/client";
import { IconAction } from "../../components/IconAction";
import { GoogleContext } from "../../google/useGoogle";
import { pickDriveFile } from "../../lib/googlePicker";

export interface UploadReportDialogProps {
  opened: boolean;
  onClose: () => void;
  onUploaded: (report: ReportBody) => void;
  nre?: string;
  defaultTitle?: string;
  defaultDate?: string;
  defaultProvider?: string;
}

type Metadata = Pick<ImportDriveRequest, "title" | "date" | "provider" | "nre">;

const accept = "application/pdf,image/jpeg,image/png";
const maxBytes = 20 * 1024 * 1024;
const tooLargeMessage = "Il file supera i 20 MB";
const unsupportedMessage = "Formato non supportato o file non valido";
const failedMessage = "Caricamento non riuscito";
const pickerNotConfiguredMessage = "Aggiungi la API key in Impostazioni Google";
const pickerExpiredMessage =
  "Accesso a Google scaduto: premi di nuovo Scegli da Google Drive";
const pickerFailedMessage = "Impossibile aprire Google Drive";

function uploadErrorMessage(status: number): string {
  if (status === 413) {
    return tooLargeMessage;
  }
  if (status === 422) {
    return unsupportedMessage;
  }
  return failedMessage;
}

function pickerErrorMessage(status: number): string {
  if (status === 409) {
    return pickerNotConfiguredMessage;
  }
  if (status === 401) {
    return pickerExpiredMessage;
  }
  return pickerFailedMessage;
}

function FormBody({
  onClose,
  onUploaded,
  nre,
  defaultTitle,
  defaultDate,
  defaultProvider,
}: UploadReportDialogProps): JSX.Element {
  const google = useContext(GoogleContext);
  const [file, setFile] = useState<File | null>(null);
  const [driveFile, setDriveFile] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [title, setTitle] = useState(defaultTitle ?? "");
  const [date, setDate] = useState(defaultDate ?? "");
  const [provider, setProvider] = useState(defaultProvider ?? "");
  const [error, setError] = useState<string | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const shownError = error ?? google?.error;

  const metadata: Metadata = {
    ...(title.trim() === "" ? {} : { title: title.trim() }),
    ...(date === "" ? {} : { date }),
    ...(provider.trim() === "" ? {} : { provider: provider.trim() }),
    ...(nre === undefined ? {} : { nre }),
  };

  const finish = (report: ReportBody): void => {
    onUploaded(report);
    onClose();
  };

  const pickFile = (selected: File | null): void => {
    setFile(selected);
    if (selected !== null) {
      setDriveFile(null);
    }
    if (selected !== null && selected.size > maxBytes) {
      setError(tooLargeMessage);
      return;
    }
    setError(undefined);
  };

  const upload = async (): Promise<void> => {
    if (file === null || file.size > maxBytes) {
      return;
    }
    const form = new FormData();
    form.append("file", file);
    for (const [key, value] of Object.entries(metadata)) {
      form.append(key, value);
    }
    setBusy(true);
    setError(undefined);
    try {
      const { data, response } = await api.POST("/api/reports/local", {
        body: { file: "" },
        bodySerializer: () => form,
      });
      if (data === undefined) {
        setError(uploadErrorMessage(response.status));
        return;
      }
      finish(data);
    } catch {
      setError(failedMessage);
    } finally {
      setBusy(false);
    }
  };

  const importFromDrive = async (): Promise<void> => {
    setBusy(true);
    setError(undefined);
    try {
      if (google?.status?.connected !== true) {
        const signedIn = (await google?.signIn()) ?? false;
        if (!signedIn) {
          return;
        }
      }
      const picker = await api.POST("/api/google/picker");
      if (picker.data === undefined) {
        if (picker.response.status === 401) {
          google?.refresh();
        }
        setError(pickerErrorMessage(picker.response.status));
        return;
      }
      const picked = await pickDriveFile(picker.data);
      if (picked === null) {
        return;
      }
      setDriveFile(picked);
      setFile(null);
    } catch {
      setError(pickerFailedMessage);
    } finally {
      setBusy(false);
    }
  };

  const uploadFromDrive = async (id: string): Promise<void> => {
    setBusy(true);
    setError(undefined);
    try {
      const { data, response } = await api.POST("/api/reports/local/drive", {
        body: { fileId: id, ...metadata },
      });
      if (data === undefined) {
        setError(uploadErrorMessage(response.status));
        return;
      }
      finish(data);
    } catch {
      setError(failedMessage);
    } finally {
      setBusy(false);
    }
  };

  const handleSubmit = (event: FormSubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    if (driveFile !== null) {
      void uploadFromDrive(driveFile.id);
      return;
    }
    void upload();
  };

  return (
    <form onSubmit={handleSubmit} noValidate>
      <Stack gap="sm">
        <Group align="flex-end" gap="xs" wrap="nowrap">
          <FileInput
            label="File"
            description="PDF, JPEG o PNG, massimo 20 MB"
            accept={accept}
            clearable
            value={file}
            onChange={pickFile}
            style={{ flex: 1 }}
          />
          {google?.status?.configured === true && google.status.drive ? (
            <IconAction
              label="Scegli da Google Drive"
              icon={HardDrive}
              disabled={busy}
              onClick={() => {
                void importFromDrive();
              }}
            />
          ) : null}
        </Group>
        {driveFile === null ? null : (
          <Group gap="xs" wrap="nowrap">
            <HardDrive size={16} />
            <Text size="sm" truncate style={{ flex: 1 }}>
              {driveFile.name}
            </Text>
            <IconAction
              label="Rimuovi file di Drive"
              icon={X}
              onClick={() => {
                setDriveFile(null);
              }}
            />
          </Group>
        )}
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
        {shownError === undefined ? null : (
          <Text size="sm" c="red" role="alert">
            {shownError}
          </Text>
        )}
        <Group justify="flex-end" gap="xs">
          <IconAction
            label="Carica"
            icon={Upload}
            type="submit"
            tone="primary"
            loading={busy}
            disabled={
              driveFile === null && (file === null || file.size > maxBytes)
            }
          />
        </Group>
      </Stack>
    </form>
  );
}

export function UploadReportDialog(
  props: UploadReportDialogProps,
): JSX.Element {
  return (
    <Modal opened={props.opened} onClose={props.onClose} title="Carica referto">
      <FormBody {...props} />
    </Modal>
  );
}
