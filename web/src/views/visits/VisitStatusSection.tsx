import { Button, Select } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import type { JSX } from "react";

import { api, type RowBody } from "../../api/client";
import {
  prescriptionStatuses,
  type SelectableStatus,
  prescriptionStatusLabel,
} from "../../lib/status";

const options = prescriptionStatuses.map((status) => ({
  value: String(status),
  label: prescriptionStatusLabel(status),
}));

interface Props {
  row: RowBody;
  onChanged: () => void;
}

export function VisitStatusSection({ row, onChanged }: Props): JSX.Element {
  const current = row.fse?.status ?? row.cup?.status;
  const value =
    row.overridden && current !== undefined ? String(current) : null;

  const setStatus = async (status: SelectableStatus): Promise<void> => {
    try {
      const { response } = await api.PUT("/api/visits/status", {
        body: { nre: row.nre, status },
      });
      if (!response.ok) {
        notifications.show({
          color: "red",
          message: "Impossibile salvare lo stato",
        });
        return;
      }
      onChanged();
    } catch {
      notifications.show({
        color: "red",
        message: "Impossibile salvare lo stato",
      });
    }
  };

  const resetStatus = async (): Promise<void> => {
    try {
      const { response } = await api.DELETE("/api/visits/status", {
        body: { nre: row.nre },
      });
      if (!response.ok) {
        notifications.show({
          color: "red",
          message: "Impossibile ripristinare lo stato",
        });
        return;
      }
      onChanged();
    } catch {
      notifications.show({
        color: "red",
        message: "Impossibile ripristinare lo stato",
      });
    }
  };

  return (
    <>
      <Select
        label="Imposta stato a mano"
        placeholder="Stato di Toscana Salute"
        data={options}
        value={value}
        allowDeselect={false}
        onChange={(next) => {
          const status = prescriptionStatuses.find(
            (item) => String(item) === next,
          );
          if (status !== undefined) {
            void setStatus(status);
          }
        }}
      />
      {row.overridden ? (
        <Button
          variant="light"
          w="fit-content"
          onClick={() => {
            void resetStatus();
          }}
        >
          Usa lo stato di Toscana Salute
        </Button>
      ) : null}
    </>
  );
}
