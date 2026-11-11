import { Box, Group, Table, Text } from "@mantine/core";
import type { CSSProperties } from "react";
import { CalendarCheck, MapPin } from "lucide-react";
import { useState, type JSX } from "react";

import type { AppointmentBody, RowBody } from "../../api/client";
import { CopyButton } from "../../components/CopyButton";
import { IconAction } from "../../components/IconAction";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { formatDate, formatTime } from "../../lib/format";
import { useFocusTarget } from "../../lib/useFocusTarget";
import { BookAction } from "./BookAction";
import { unitName, rowDate, rowTitle, type EffectiveState } from "./filter";
import { CalendarDialog } from "./CalendarDialog";
import { mapUrl } from "./mapUrl";
import { PromemoriaLink } from "./PromemoriaLink";
import { relativeDay } from "./relativeDay";

const nowrap: CSSProperties = { whiteSpace: "nowrap" };
const focusedBg = "var(--mantine-primary-color-light)";
const slotSize = 34;

interface Props {
  row: RowBody;
  state: EffectiveState;
  booking: AppointmentBody | undefined;
  appointmentsLoading: boolean;
  appointmentsFailed: boolean;
  now: Date;
}

export function VisitRow({
  row,
  state,
  booking,
  appointmentsLoading,
  appointmentsFailed,
  now,
}: Props): JSX.Element {
  const { id, ref, tabIndex, focused } = useFocusTarget<HTMLTableRowElement>(
    "visite",
    row.nre,
  );
  const at = state.kind === "booked" ? state.at : undefined;
  const unit = state.kind === "booked" ? booking?.unit : undefined;
  const [calendarOpen, setCalendarOpen] = useState(false);
  const calendarBooking =
    booking !== undefined &&
    at !== undefined &&
    new Date(at).getTime() > now.getTime()
      ? booking
      : undefined;
  const bookingId = state.kind === "booked" ? booking?.bookingId : undefined;
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
  const place =
    unit === undefined
      ? ""
      : [unit.address, unit.city, unit.province]
          .filter((part) => part !== "")
          .join(", ");

  let appointmentCell: JSX.Element | null = null;
  if (at !== undefined) {
    appointmentCell = (
      <>
        {formatDate(at)} {formatTime(at)}
        <Text size="xs" c="dimmed">
          {relativeDay(at, now)}
        </Text>
      </>
    );
  } else if (appointmentsLoading) {
    appointmentCell = <>ricerca…</>;
  } else if (appointmentsFailed) {
    appointmentCell = <>—</>;
  }

  return (
    <Table.Tr
      id={id}
      ref={ref}
      tabIndex={tabIndex}
      data-focused={focused ? "true" : undefined}
      {...(focused ? { bg: focusedBg } : {})}
    >
      <Table.Td style={nowrap}>{formatDate(rowDate(row))}</Table.Td>
      <Table.Td>{rowTitle(row)}</Table.Td>
      <Table.Td>
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
      </Table.Td>
      <Table.Td style={nowrap} aria-busy={appointmentsLoading}>
        {appointmentCell}
      </Table.Td>
      <Table.Td>
        {unit === undefined ? null : (
          <>
            {unitName(unit)}
            {place === "" ? null : (
              <Text size="xs" c="dimmed">
                {place}
              </Text>
            )}
            <IconAction
              label="Apri su mappa"
              icon={MapPin}
              href={mapUrl(unit.address, unit.city)}
              external
            />
          </>
        )}
      </Table.Td>
      <Table.Td style={nowrap}>
        <CopyButton value={row.nre} label="Copia NRE" />
      </Table.Td>
      <Table.Td>
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
            {state.kind === "official" && state.status === 3 ? (
              <BookAction />
            ) : null}
          </Box>
        </Group>
      </Table.Td>
    </Table.Tr>
  );
}
