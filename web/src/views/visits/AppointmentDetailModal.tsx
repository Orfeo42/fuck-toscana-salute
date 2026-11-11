import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { MapPinned, Navigation } from "lucide-react";
import type { JSX } from "react";

import type { AppointmentBody, RowBody } from "../../api/client";
import { CopyButton } from "../../components/CopyButton";
import { StatusBadge } from "../../components/StatusBadge";
import { formatLongDateTime } from "../../lib/format";
import { rowTitle, unitName, type EffectiveState } from "./filter";
import { mapsDirectionsUrl, mapsSearchUrl } from "./mapsUrl";
import { relativeDay } from "./relativeDay";

interface Props {
  row: RowBody;
  state: EffectiveState;
  booking: AppointmentBody;
  now: Date;
  onClose: () => void;
}

interface FieldProps {
  label: string;
  children: JSX.Element | string;
}

function Field({ label, children }: FieldProps): JSX.Element {
  return (
    <div>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      {typeof children === "string" ? <Text>{children}</Text> : children}
    </div>
  );
}

export function AppointmentDetailModal({
  row,
  state,
  booking,
  now,
  onClose,
}: Props): JSX.Element {
  const fullScreen = useMediaQuery("(max-width: 48em)");
  const { unit } = booking;
  const name = unitName(unit);
  const place = [unit.address, unit.city, unit.province]
    .filter((part) => part !== "")
    .join(", ");
  const hasAddress = unit.address !== "";
  const at = state.kind === "booked" ? state.at : undefined;

  return (
    <Modal
      opened
      onClose={onClose}
      title="Appuntamento"
      fullScreen={fullScreen}
      centered
    >
      <Stack gap="md">
        <Group
          gap="sm"
          justify="space-between"
          wrap="nowrap"
          align="flex-start"
        >
          <Text fw={600}>{rowTitle(row)}</Text>
          <StatusBadge tone={state.tone}>{state.label}</StatusBadge>
        </Group>
        {at === undefined ? null : (
          <Field label="Data e ora">
            <>
              <Text>{formatLongDateTime(at)}</Text>
              <Text size="sm" c="dimmed">
                {relativeDay(at, now)}
              </Text>
            </>
          </Field>
        )}
        <Field label="Struttura">
          <>
            <Text>{name}</Text>
            {unit.description !== "" && unit.description !== unit.name ? (
              <Text size="sm" c="dimmed">
                {unit.name}
              </Text>
            ) : null}
            {place === "" ? null : <Text size="sm">{place}</Text>}
          </>
        </Field>
        <Field label="NRE">
          <CopyButton value={row.nre} label="Copia NRE" />
        </Field>
        {hasAddress ? (
          <Group gap="sm">
            <Button
              component="a"
              href={mapsDirectionsUrl(unit.address, unit.city)}
              target="_blank"
              rel="noopener noreferrer"
              leftSection={<Navigation size={16} aria-hidden="true" />}
            >
              Indicazioni stradali
            </Button>
            <Button
              component="a"
              variant="light"
              href={mapsSearchUrl(unit.address, unit.city)}
              target="_blank"
              rel="noopener noreferrer"
              leftSection={<MapPinned size={16} aria-hidden="true" />}
            >
              Apri in Google Maps
            </Button>
          </Group>
        ) : null}
      </Stack>
    </Modal>
  );
}
