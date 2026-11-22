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
import { Info, MapPin } from "lucide-react";
import { useState, type JSX } from "react";

import type { AppointmentBody, RowBody } from "../../api/client";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { StopClick } from "../../components/StopClick";
import { formatCompactDateTime } from "../../lib/format";
import { BookAction } from "./BookAction";
import { CalendarAction } from "./CalendarAction";
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
      <StatusBadge tone={state.tone} manual={state.manual}>
        {state.label}
      </StatusBadge>
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
}

export function ActionsCell({
  row,
  state,
  booking,
  now,
}: ActionsProps): JSX.Element {
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
          <CalendarAction appointment={calendarBooking} title={rowTitle(row)} />
        )}
      </Box>
      <Box w={slotSize} h={slotSize}>
        <PdfLink documentRef={row.fse?.documentRef} />
      </Box>
      <Box w={slotSize} h={slotSize}>
        {state.kind === "official" && state.canBook ? <BookAction /> : null}
      </Box>
    </Group>
  );
}
