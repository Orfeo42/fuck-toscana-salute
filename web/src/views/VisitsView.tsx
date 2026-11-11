import {
  Alert,
  Button,
  Group,
  Loader,
  ScrollArea,
  SegmentedControl,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { RefreshCw, Search } from "lucide-react";
import { useState } from "react";
import type { JSX } from "react";

import { SortHeader } from "../components/SortHeader";
import { useHealthData } from "../data/useHealthData";
import { useRoute } from "../lib/route";
import { useSort } from "../lib/useSort";
import { VisitRow } from "./visits/VisitRow";
import {
  buildRows,
  countByStatus,
  effectiveState,
  filterRows,
  sortRows,
  type StatusFilter,
  type VisitSortKey,
} from "./visits/filter";

const statusOptions: readonly { value: StatusFilter; label: string }[] = [
  { value: "upcoming", label: "Prossimi" },
  { value: "bookable", label: "Da prenotare" },
  { value: "open", label: "Aperte" },
  { value: "done", label: "Erogate" },
  { value: "expired", label: "Scadute" },
  { value: "all", label: "Tutte" },
];

const descFirst: readonly VisitSortKey[] = ["prescritta"];

interface Props {
  now?: Date;
}

export function VisitsView({ now }: Props = {}): JSX.Element {
  const health = useHealthData();
  const { prescriptions, appointments, bookingByNre, unbooked } = health;
  const route = useRoute();
  const focus = route.tab === "visite" ? route.focus : undefined;
  const [mountedAt] = useState(() => new Date());
  const [handledFocus, setHandledFocus] = useState<string | undefined>(
    undefined,
  );
  const [status, setStatus] = useState<StatusFilter>("open");
  const [search, setSearch] = useState("");
  const sort = useSort<VisitSortKey>("prescritta", "desc", descFirst);

  const currentNow = now ?? mountedAt;
  const context = { bookingByNre, unbooked, now: currentNow };
  const data = buildRows(
    prescriptions.data?.prescriptions ?? [],
    appointments.data?.appointments ?? [],
  );
  const counts = countByStatus(data, search, context);
  const visible = sortRows(
    filterRows(data, { status, search }, context),
    sort.key,
    sort.dir,
    context,
  );

  if (
    prescriptions.data !== undefined &&
    !appointments.loading &&
    focus !== handledFocus
  ) {
    setHandledFocus(focus);
    const focused = data.find((row) => row.nre.toUpperCase() === focus);
    const shown = visible.some((row) => row.nre.toUpperCase() === focus);
    if (focused !== undefined && !shown) {
      setStatus("all");
      setSearch("");
    }
  }

  const reload = (): void => {
    prescriptions.reload();
    appointments.reload();
  };

  const segmentData = statusOptions.map((option) => ({
    value: option.value,
    label: `${option.label} (${String(counts[option.value])})`,
  }));

  const summary = `${String(visible.length)} visite/esami`;
  const ready = !prescriptions.loading && prescriptions.error === undefined;

  return (
    <section aria-labelledby="visits-title" aria-busy={prescriptions.loading}>
      <Title order={2} id="visits-title">
        Visite ed esami
      </Title>
      <Group align="flex-end" wrap="wrap" mb="md">
        <Stack gap={4} maw="100%">
          <Text id="vs-status-label" size="sm">
            Mostra
          </Text>
          <ScrollArea type="never" maw="100%">
            <SegmentedControl
              aria-labelledby="vs-status-label"
              data={segmentData}
              value={status}
              onChange={(value) => {
                const picked = statusOptions.find(
                  (option) => option.value === value,
                );
                if (picked !== undefined) {
                  setStatus(picked.value);
                }
              }}
            />
          </ScrollArea>
        </Stack>
        <TextInput
          label="Cerca"
          type="search"
          leftSection={<Search size={16} aria-hidden="true" />}
          value={search}
          onChange={(event) => {
            setSearch(event.currentTarget.value);
          }}
        />
        <Text aria-live="polite">{summary}</Text>
        <Button
          variant="light"
          leftSection={<RefreshCw size={16} aria-hidden="true" />}
          onClick={reload}
        >
          Aggiorna
        </Button>
      </Group>
      {prescriptions.loading ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text>Carico visite ed esami…</Text>
        </Group>
      ) : null}
      {!prescriptions.loading && prescriptions.error === "unauthorized" ? (
        <Alert color="yellow" role="alert" mb="md">
          Accedi per vedere visite ed esami.
        </Alert>
      ) : null}
      {!prescriptions.loading &&
      prescriptions.error !== undefined &&
      prescriptions.error !== "unauthorized" ? (
        <Alert color="red" role="alert" mb="md">
          Il portale regionale non risponde. Riprova tra poco.
        </Alert>
      ) : null}
      {ready && appointments.error === "unauthorized" ? (
        <Alert color="yellow" role="alert" mb="md">
          Accedi per vedere gli appuntamenti.
        </Alert>
      ) : null}
      {ready && appointments.error === "needs_gestione" ? (
        <Alert color="yellow" role="alert" mb="md">
          Per vedere date e orari serve l&apos;accesso a Gestione Prenotazioni:
          premi Accedi.
        </Alert>
      ) : null}
      {ready && appointments.error === "unavailable" ? (
        <Alert color="red" role="alert" mb="md">
          Appuntamenti non disponibili: il portale regionale non risponde.
        </Alert>
      ) : null}
      {ready && visible.length === 0 ? (
        <Text c="dimmed">Nessuna visita o esame in questa vista.</Text>
      ) : null}
      {ready && visible.length > 0 ? (
        <Table.ScrollContainer minWidth={1000}>
          <Table striped highlightOnHover stickyHeader verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <SortHeader
                  label="Prescritta"
                  sortKey="prescritta"
                  sort={sort}
                />
                <SortHeader
                  label="Prestazione"
                  sortKey="prestazione"
                  sort={sort}
                />
                <SortHeader label="Stato" sortKey="stato" sort={sort} />
                <SortHeader
                  label="Appuntamento"
                  sortKey="appuntamento"
                  sort={sort}
                />
                <SortHeader label="Struttura" sortKey="struttura" sort={sort} />
                <SortHeader label="NRE" sortKey="nre" sort={sort} />
                <Table.Th scope="col">Azioni</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {visible.map((row) => (
                <VisitRow
                  key={row.nre}
                  row={row}
                  state={effectiveState(
                    row,
                    bookingByNre,
                    unbooked,
                    currentNow,
                  )}
                  booking={bookingByNre.get(row.nre.toUpperCase())}
                  appointmentsLoading={appointments.loading}
                  appointmentsFailed={appointments.error !== undefined}
                  now={currentNow}
                />
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      ) : null}
    </section>
  );
}
