import { Group, Select } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { RotateCcw } from "lucide-react";
import type { JSX } from "react";

import {
  api,
  type ManualStatusBody,
  type VisitStatusRequest,
} from "../../api/client";
import { IconAction } from "../../components/IconAction";

type SelectableStatus = VisitStatusRequest["status"];

const selectable: Readonly<Record<SelectableStatus, true>> = {
  3: true,
  30: true,
  5: true,
  6: true,
  7: true,
  8: true,
  9: true,
  4: true,
};

function isSelectable(code: number): code is SelectableStatus {
  return Object.hasOwn(selectable, code);
}

interface Props {
  nre: string;
  manualStatus: ManualStatusBody;
  onChanged: () => void;
}

export function VisitStatusSection({
  nre,
  manualStatus,
  onChanged,
}: Props): JSX.Element {
  const { current, options } = manualStatus;
  const data = options.map((option) => ({
    value: String(option.code),
    label: option.label,
  }));

  const setStatus = async (status: SelectableStatus): Promise<void> => {
    try {
      const { response } = await api.PUT("/api/visits/status", {
        body: { nre, status },
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
        body: { nre },
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
        data={data}
        value={current === null ? null : String(current)}
        allowDeselect={false}
        onChange={(next) => {
          const code = options.find(
            (option) => String(option.code) === next,
          )?.code;
          if (code !== undefined && isSelectable(code)) {
            void setStatus(code);
          }
        }}
      />
      {current === null ? null : (
        <Group>
          <IconAction
            label="Usa lo stato di Toscana Salute"
            icon={RotateCcw}
            onClick={() => {
              void resetStatus();
            }}
          />
        </Group>
      )}
    </>
  );
}
