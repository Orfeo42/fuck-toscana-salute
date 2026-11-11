import {
  ActionIcon,
  Box,
  Group,
  Popover,
  Stack,
  Text,
  Tooltip,
  UnstyledButton,
} from "@mantine/core";
import { CalendarCheck, Info, MapPin } from "lucide-react";
import { useState, type JSX } from "react";

import type { AppointmentBody, RowBody } from "../../api/client";
import { IconAction } from "../../components/IconAction";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { StopClick } from "../../components/StopClick";
import { formatCompactDateTime } from "../../lib/format";
import { BookAction } from "./BookAction";
import { CalendarDialog } from "./CalendarDialog";
import { unitName, rowTitle, type EffectiveState } from "./filter";
import { mapsSearchUrl } from "./mapsUrl";
import { PromemoriaLink } from "./PromemoriaLink";
import { statusDetails } from "./statusDetails";

const slotSize = 34;
const infoWidth = 280;

interface InfoProps {
  details: readonly string[];
}

function StatusInfo({ details }: InfoProps): JSX.Element {
  const [opened, setOpened] = useState(false);
  const lines = (
    <Stack gap={4}>
      {details.map((detail) => (
        <Text key={detail} size="sm">
          {detail}
        </Text>
      ))}
    </Stack>
  );
  return (
    <StopClick>
      <Popover
        width={infoWidth}
        position="bottom"
        withArrow
        shadow="md"
        onChange={setOpened}
      >
        <Tooltip
          label={lines}
          withArrow
          multiline
          w={infoWidth}
          openDelay={150}
          disabled={opened}
        >
          <Popover.Target>
            <ActionIcon variant="subtle" size="sm" aria-label="Dettagli stato">
              <Info size={16} aria-hidden="true" />
            </ActionIcon>
          </Popover.Target>
        </Tooltip>
        <Popover.Dropdown>{lines}</Popover.Dropdown>
      </Popover>
    </StopClick>
  );
}

interface StatusProps {
  row: RowBody;
  state: EffectiveState;
}

export function StatusCell({ row, state }: StatusProps): JSX.Element {
  const details = statusDetails(row, state);
  return (
    <Group gap={4} wrap="nowrap">
      <StatusBadge tone={state.tone}>{state.label}</StatusBadge>
      {details.length === 0 ? null : <StatusInfo details={details} />}
    </Group>
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

interface PlaceProps {
  state: EffectiveState;
  booking: AppointmentBody | undefined;
}

export function PlaceCell({ state, booking }: PlaceProps): JSX.Element | null {
  const unit = state.kind === "booked" ? booking?.unit : undefined;
  if (unit === undefined || unit.address === "") {
    return null;
  }
  const name = unitName(unit);
  return (
    <StopClick>
      <Tooltip label={name} withArrow openDelay={150}>
        <ActionIcon
          component="a"
          variant="subtle"
          size="lg"
          aria-label={`Apri in Google Maps: ${name}`}
          href={mapsSearchUrl(unit)}
          target="_blank"
          rel="noopener noreferrer"
        >
          <MapPin size={18} aria-hidden="true" />
        </ActionIcon>
      </Tooltip>
    </StopClick>
  );
}

interface ActionsProps {
  row: RowBody;
  state: EffectiveState;
  booking: AppointmentBody | undefined;
  now: Date;
  labelled?: boolean;
}

export function ActionsCell({
  row,
  state,
  booking,
  now,
  labelled = false,
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

  const promemoria = (
    <PromemoriaLink bookingId={bookingId} labelled={labelled} />
  );
  const calendar =
    calendarBooking === undefined ? null : (
      <IconAction
        label="Aggiungi a Google Calendar"
        icon={CalendarCheck}
        labelled={labelled}
        onClick={() => {
          setCalendarOpen(true);
        }}
      />
    );
  const calendarDialog =
    calendarBooking !== undefined && calendarOpen ? (
      <CalendarDialog
        appointment={calendarBooking}
        title={rowTitle(row)}
        onClose={() => {
          setCalendarOpen(false);
        }}
      />
    ) : null;
  const pdf = (
    <PdfLink documentRef={row.fse?.documentRef} labelled={labelled} />
  );
  const book =
    state.kind === "official" && state.canBook ? (
      <BookAction labelled={labelled} />
    ) : null;

  if (labelled) {
    return (
      <Group gap="sm" wrap="wrap">
        {promemoria}
        {calendar}
        {calendarDialog}
        {pdf}
        {book}
      </Group>
    );
  }

  return (
    <Group gap={2} wrap="nowrap">
      <Box w={slotSize} h={slotSize}>
        {promemoria}
      </Box>
      <Box w={slotSize} h={slotSize}>
        {calendar}
      </Box>
      {calendarDialog}
      <Box w={slotSize} h={slotSize}>
        {pdf}
      </Box>
      <Box w={slotSize} h={slotSize}>
        {book}
      </Box>
    </Group>
  );
}
