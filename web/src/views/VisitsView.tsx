import {
  Alert,
  Button,
  Group,
  Loader,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { RefreshCw, Search } from "lucide-react";
import { DataTable, type DataTableColumn } from "mantine-datatable";
import { useState } from "react";
import type { JSX, KeyboardEvent } from "react";

import { CopyButton } from "../components/CopyButton";
import { StopClick } from "../components/StopClick";
import { StatusColumnFilter } from "../components/StatusColumnFilter";
import { useHealthData } from "../data/useHealthData";
import { formatDate, formatUpdated } from "../lib/format";
import { tableColors } from "../theme";
import { focusId, useFocusTarget } from "../lib/useFocusTarget";
import { useSort } from "../lib/useSort";
import type { RowBody } from "../api/client";
import {
  ActionsCell,
  AppointmentCell,
  PlaceCell,
  StatusCell,
} from "./visits/VisitCells";
import { VisitDetailModal } from "./visits/VisitDetailModal";
import {
  buildRows,
  defaultExcludedLabels,
  effectiveState,
  excludeLabels,
  labelCounts,
  rowDate,
  rowLabel,
  rowTitle,
  searchRows,
  sortRows,
  visitSortKeys,
  type VisitSortKey,
} from "./visits/filter";

const descFirst: readonly VisitSortKey[] = ["prescritta"];
const focusedBg = "var(--mantine-primary-color-light)";
const rowPointer = { cursor: "pointer" };

interface Props {
  now?: Date;
}

export function VisitsView({ now }: Props = {}): JSX.Element {
  const health = useHealthData();
  const {
    prescriptions,
    appointments,
    bookingByNre,
    unbooked,
    verdictByNre,
    appointmentCheckedAtByNre,
    refresh,
    refreshing,
  } = health;
  const [mountedAt] = useState(() => new Date());
  const [handledFocus, setHandledFocus] = useState<string | undefined>(
    undefined,
  );
  const [excluded, setExcluded] = useState<readonly string[]>(
    defaultExcludedLabels,
  );
  const [search, setSearch] = useState("");
  const [detailNre, setDetailNre] = useState<string | undefined>(undefined);
  const [selectedNres, setSelectedNres] = useState<readonly string[]>([]);
  const sort = useSort<VisitSortKey>("prescritta", "desc", descFirst);

  const currentNow = now ?? mountedAt;
  const context = { bookingByNre, unbooked, verdictByNre, now: currentNow };
  const dataReady = prescriptions.data !== undefined && !appointments.loading;
  const focus = useFocusTarget("visite", dataReady);
  const data = buildRows(
    prescriptions.data?.prescriptions ?? [],
    appointments.data?.appointments ?? [],
  );
  const searched = searchRows(data, search, context);
  const options = labelCounts(searched, context);
  const visible = sortRows(
    excludeLabels(searched, excluded, context),
    sort.key,
    sort.dir,
    context,
  );

  if (dataReady && focus !== handledFocus) {
    setHandledFocus(focus);
    const focused = data.find((row) => row.nre.toUpperCase() === focus);
    const shown = visible.some((row) => row.nre.toUpperCase() === focus);
    if (focused !== undefined && !shown) {
      const label = rowLabel(focused, context);
      setExcluded(excluded.filter((item) => item !== label));
      setSearch("");
    }
  }

  const columns: DataTableColumn<RowBody>[] = [
    {
      accessor: "prescritta",
      title: "Prescritta",
      sortable: true,
      noWrap: true,
      render: (row) => formatDate(rowDate(row)),
    },
    {
      accessor: "prestazione",
      title: "Prestazione",
      sortable: true,
      render: (row) => rowTitle(row),
    },
    {
      accessor: "stato",
      title: "Stato",
      sortable: true,
      filtering: excluded.length > 0,
      filter: (
        <StatusColumnFilter
          options={options.map((option) => ({
            value: option.label,
            label: option.label,
            count: option.count,
          }))}
          selected={options
            .map((option) => option.label)
            .filter((label) => !excluded.includes(label))}
          onChange={(next) => {
            setExcluded([
              ...excluded.filter(
                (label) => !options.some((option) => option.label === label),
              ),
              ...options
                .map((option) => option.label)
                .filter((label) => !next.includes(label)),
            ]);
          }}
        />
      ),
      render: (row) => (
        <StatusCell
          row={row}
          state={effectiveState(
            row,
            bookingByNre,
            unbooked,
            currentNow,
            verdictByNre,
          )}
        />
      ),
    },
    {
      accessor: "appuntamento",
      title: "Appuntamento",
      sortable: true,
      noWrap: true,
      width: 170,
      customCellAttributes: () => ({ "aria-busy": appointments.loading }),
      render: (row) => (
        <AppointmentCell
          state={effectiveState(
            row,
            bookingByNre,
            unbooked,
            currentNow,
            verdictByNre,
          )}
          loading={appointments.loading}
          failed={appointments.error !== undefined}
          onOpen={() => {
            setDetailNre(row.nre);
          }}
        />
      ),
    },
    {
      accessor: "struttura",
      title: "Luogo",
      sortable: true,
      width: 96,
      textAlign: "center",
      render: (row) => (
        <PlaceCell
          state={effectiveState(
            row,
            bookingByNre,
            unbooked,
            currentNow,
            verdictByNre,
          )}
          booking={bookingByNre.get(row.nre.toUpperCase())}
        />
      ),
    },
    {
      accessor: "nre",
      title: "NRE",
      sortable: true,
      noWrap: true,
      render: (row) => (
        <StopClick>
          <CopyButton value={row.nre} label="Copia NRE" />
        </StopClick>
      ),
    },
    {
      accessor: "azioni",
      title: "Azioni",
      render: (row) => (
        <StopClick>
          <ActionsCell
            row={row}
            state={effectiveState(
              row,
              bookingByNre,
              unbooked,
              currentNow,
              verdictByNre,
            )}
            booking={bookingByNre.get(row.nre.toUpperCase())}
            now={currentNow}
          />
        </StopClick>
      ),
    },
  ];

  const detailRow =
    detailNre === undefined
      ? undefined
      : data.find((row) => row.nre === detailNre);

  const selected = visible.filter((row) => selectedNres.includes(row.nre));
  const updatedText = formatUpdated(
    appointments.data?.updated.lists,
    currentNow,
  );
  const summary = `${String(visible.length)} visite/esami`;
  const ready = !prescriptions.loading && prescriptions.error === undefined;

  return (
    <section aria-labelledby="visits-title" aria-busy={prescriptions.loading}>
      <Title order={2} id="visits-title">
        Visite ed esami
      </Title>
      <Group align="flex-end" wrap="wrap" mb="md">
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
        {updatedText === undefined ? null : (
          <Text size="sm" c="dimmed">
            {updatedText}
          </Text>
        )}
        <Button
          variant="light"
          loading={refreshing}
          leftSection={<RefreshCw size={16} aria-hidden="true" />}
          onClick={() => {
            void refresh({ all: true });
          }}
        >
          Aggiorna tutto
        </Button>
        {selected.length === 0 ? null : (
          <Button
            variant="light"
            loading={refreshing}
            leftSection={<RefreshCw size={16} aria-hidden="true" />}
            onClick={() => {
              const nres = [
                ...new Set(
                  selected
                    .map((row) => row.nre.toUpperCase())
                    .filter((nre) => nre !== ""),
                ),
              ];
              setSelectedNres([]);
              void refresh({ nres });
            }}
          >
            {`Aggiorna selezionate (${String(selected.length)})`}
          </Button>
        )}
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
      {ready ? (
        <DataTable
          striped
          highlightOnHover
          withTableBorder
          {...tableColors}
          maxHeight="75vh"
          idAccessor="nre"
          records={visible}
          selectedRecords={selected}
          onSelectedRecordsChange={(records) => {
            setSelectedNres(records.map((row) => row.nre));
          }}
          columns={columns}
          noRecordsText="Nessuna visita o esame in questa vista."
          sortStatus={{ columnAccessor: sort.key, direction: sort.dir }}
          onSortStatusChange={(status) => {
            const key = visitSortKeys.find(
              (item) => item === status.columnAccessor,
            );
            if (key !== undefined) {
              sort.toggle(key);
            }
          }}
          onRowClick={({ record }) => {
            setDetailNre(record.nre);
          }}
          customRowAttributes={(row) => ({
            id: focusId("visite", row.nre),
            tabIndex: 0,
            onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
              if (
                event.key === "Enter" &&
                event.target === event.currentTarget
              ) {
                setDetailNre(row.nre);
              }
            },
            "data-focused":
              row.nre.toUpperCase() === focus ? "true" : undefined,
          })}
          rowStyle={(row) =>
            row.nre.toUpperCase() === focus
              ? { ...rowPointer, backgroundColor: focusedBg }
              : rowPointer
          }
        />
      ) : null}
      {detailRow === undefined ? null : (
        <VisitDetailModal
          row={detailRow}
          state={effectiveState(
            detailRow,
            bookingByNre,
            unbooked,
            currentNow,
            verdictByNre,
          )}
          booking={bookingByNre.get(detailRow.nre.toUpperCase())}
          now={currentNow}
          appointmentCheckedAt={appointmentCheckedAtByNre.get(
            detailRow.nre.toUpperCase(),
          )}
          refreshing={refreshing}
          onRefresh={() => {
            void refresh({ nres: [detailRow.nre.toUpperCase()] });
          }}
          onClose={() => {
            setDetailNre(undefined);
          }}
        />
      )}
    </section>
  );
}
