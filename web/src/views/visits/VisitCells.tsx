import {
  ActionIcon,
  Box,
  Group,
  Loader,
  Popover,
  Stack,
  Text,
  Tooltip,
  UnstyledButton,
} from "@mantine/core";
import { Info, MapPin } from "lucide-react";
import { useState, type JSX } from "react";

import type { VisitRowBody } from "../../api/client";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { StopClick } from "../../components/StopClick";
import { formatCompactDateTime } from "../../lib/format";
import { BookAction } from "./BookAction";
import { CalendarAction } from "./CalendarAction";
import { PromemoriaLink } from "./PromemoriaLink";

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

interface RowProps {
  row: VisitRowBody;
}

export function StatusCell({ row }: RowProps): JSX.Element {
  const { status } = row;
  return (
    <Group gap={4} wrap="nowrap">
      <StatusBadge tone={status.tone} manual={status.manual}>
        {status.label}
      </StatusBadge>
      {status.details.length === 0 ? null : (
        <StatusInfo details={status.details} />
      )}
      {row.refreshing ? <Loader size="xs" aria-label="Aggiorno" /> : null}
    </Group>
  );
}

interface AppointmentProps extends RowProps {
  onOpen: () => void;
}

export function AppointmentCell({
  row,
  onOpen,
}: AppointmentProps): JSX.Element | null {
  const at = row.appointment?.at;
  if (at !== undefined) {
    return (
      <Stack gap={0}>
        <UnstyledButton aria-label="Dettagli appuntamento" onClick={onOpen}>
          <Text size="sm" span>
            {formatCompactDateTime(at)}
          </Text>
        </UnstyledButton>
        {row.appointment?.relativeDay === undefined ? null : (
          <Text size="xs" c="dimmed">
            {row.appointment.relativeDay}
          </Text>
        )}
      </Stack>
    );
  }
  if (row.appointmentLookup === "checking") {
    return <>ricerca…</>;
  }
  return row.appointmentLookup === "failed" ? <>—</> : null;
}

export function PlaceCell({ row }: RowProps): JSX.Element | null {
  const unit = row.appointment?.unit;
  if (unit === undefined || unit.address === "") {
    return null;
  }
  return (
    <StopClick>
      <Tooltip label={unit.displayName} withArrow openDelay={150}>
        <ActionIcon
          component="a"
          variant="subtle"
          size="lg"
          aria-label={`Apri in Google Maps: ${unit.displayName}`}
          href={unit.mapsSearchUrl}
          target="_blank"
          rel="noopener noreferrer"
        >
          <MapPin size={18} aria-hidden="true" />
        </ActionIcon>
      </Tooltip>
    </StopClick>
  );
}

export function ActionsCell({ row }: RowProps): JSX.Element {
  const { appointment } = row;
  return (
    <Group gap={2} wrap="nowrap">
      <Box w={slotSize} h={slotSize}>
        <PromemoriaLink bookingId={appointment?.bookingId} />
      </Box>
      <Box w={slotSize} h={slotSize}>
        {appointment?.upcoming === true ? (
          <CalendarAction appointment={appointment} />
        ) : null}
      </Box>
      <Box w={slotSize} h={slotSize}>
        <PdfLink documentRef={row.documentRef} />
      </Box>
      <Box w={slotSize} h={slotSize}>
        {row.status.canBook ? <BookAction /> : null}
      </Box>
    </Group>
  );
}
