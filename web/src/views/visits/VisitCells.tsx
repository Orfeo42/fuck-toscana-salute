import { Box, Group, Text, Tooltip, UnstyledButton } from "@mantine/core";
import { CalendarCheck } from "lucide-react";
import { useState, type JSX } from "react";

import type { AppointmentBody, RowBody } from "../../api/client";
import { IconAction } from "../../components/IconAction";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { formatCompactDateTime, formatDate } from "../../lib/format";
import { BookAction } from "./BookAction";
import { CalendarDialog } from "./CalendarDialog";
import { unitName, rowTitle, type EffectiveState } from "./filter";
import { PromemoriaLink } from "./PromemoriaLink";

const slotSize = 34;

interface StatusProps {
  row: RowBody;
  state: EffectiveState;
}

export function StatusCell({ row, state }: StatusProps): JSX.Element {
  const expiresOn = row.fse?.expiresOn;
  const estimatedSuffix = row.fse?.expiryEstimated === true ? " (stimata)" : "";
  let expiryNote: string | null = null;
  if (expiresOn !== undefined && state.kind === "expired") {
    expiryNote = `scaduta il ${formatDate(expiresOn)}${estimatedSuffix}`;
  } else if (
    expiresOn !== undefined &&
    state.kind === "official" &&
    state.status === 3
  ) {
    expiryNote = `scade il ${formatDate(expiresOn)}${estimatedSuffix}`;
  }
  return (
    <>
      <StatusBadge tone={state.tone}>{state.label}</StatusBadge>
      {state.kind === "official" && state.note !== undefined ? (
        <Text size="xs" c="dimmed">
          {state.note}
        </Text>
      ) : null}
      {expiryNote === null ? null : (
        <Text size="xs" c="dimmed">
          {expiryNote}
        </Text>
      )}
    </>
  );
}

interface AppointmentProps {
  state: EffectiveState;
  loading: boolean;
  failed: boolean;
  onOpen: () => void;
}

export function AppointmentCell({
  state,
  loading,
  failed,
  onOpen,
}: AppointmentProps): JSX.Element | null {
  const at = state.kind === "booked" ? state.at : undefined;
  if (at !== undefined) {
    return (
      <UnstyledButton aria-label="Dettagli appuntamento" onClick={onOpen}>
        <Text size="sm" span>
          {formatCompactDateTime(at)}
        </Text>
      </UnstyledButton>
    );
  }
  if (loading) {
    return <>ricerca…</>;
  }
  return failed ? <>—</> : null;
}

interface StructureProps {
  state: EffectiveState;
  booking: AppointmentBody | undefined;
  onOpen: () => void;
}

export function StructureCell({
  state,
  booking,
  onOpen,
}: StructureProps): JSX.Element | null {
  const unit = state.kind === "booked" ? booking?.unit : undefined;
  if (unit === undefined) {
    return null;
  }
  const name = unitName(unit);
  return (
    <Tooltip label={name} withArrow openDelay={300}>
      <UnstyledButton
        aria-label="Dettagli appuntamento"
        onClick={onOpen}
        display="block"
        w="100%"
        ta="left"
      >
        <Text size="sm" truncate="end">
          {name}
        </Text>
      </UnstyledButton>
    </Tooltip>
  );
}

interface ActionsProps {
  row: RowBody;
  state: EffectiveState;
  booking: AppointmentBody | undefined;
  now: Date;
}

export function ActionsCell({
  row,
  state,
  booking,
  now,
}: ActionsProps): JSX.Element {
  const [calendarOpen, setCalendarOpen] = useState(false);
  const at = state.kind === "booked" ? state.at : undefined;
  const calendarBooking =
    booking !== undefined &&
    at !== undefined &&
    new Date(at).getTime() > now.getTime()
      ? booking
      : undefined;
  const bookingId = state.kind === "booked" ? booking?.bookingId : undefined;
  return (
    <Group gap={2} wrap="nowrap">
      <Box w={slotSize} h={slotSize}>
        <PromemoriaLink bookingId={bookingId} />
      </Box>
      <Box w={slotSize} h={slotSize}>
        {calendarBooking === undefined ? null : (
          <IconAction
            label="Aggiungi a Google Calendar"
            icon={CalendarCheck}
            onClick={() => {
              setCalendarOpen(true);
            }}
          />
        )}
      </Box>
      {calendarBooking !== undefined && calendarOpen ? (
        <CalendarDialog
          appointment={calendarBooking}
          title={rowTitle(row)}
          onClose={() => {
            setCalendarOpen(false);
          }}
        />
      ) : null}
      <Box w={slotSize} h={slotSize}>
        <PdfLink documentRef={row.fse?.documentRef} />
      </Box>
      <Box w={slotSize} h={slotSize}>
        {state.kind === "official" && state.canBook ? <BookAction /> : null}
      </Box>
    </Group>
  );
}
