import {
  Alert,
  Button,
  Group,
  Loader,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { RefreshCw, Search } from "lucide-react";
import { DataTable, type DataTableColumn } from "mantine-datatable";
import { useEffect, useState } from "react";
import type { JSX, KeyboardEvent } from "react";

import type {
  ReportRefBody,
  VisitRowBody,
  VisitsQueryRequest,
} from "../api/client";
import { CopyButton } from "../components/CopyButton";
import { StatusKeyFilter } from "../components/StatusKeyFilter";
import { StopClick } from "../components/StopClick";
import { tableScrollProps } from "../components/tableScroll";
import { startRefresh, type StartRefreshScope } from "../data/refresh";
import { formatDate } from "../lib/format";
import { clearRequestedVisit, useRequestedVisit } from "../lib/openVisit";
import { focusId, useFocusTarget } from "../lib/useFocusTarget";
import { useSort } from "../lib/useSort";
import { useViewQuery } from "../lib/useViewQuery";
import { tableColors } from "../theme";
import { ReportDetailModal } from "./reports/ReportDetailModal";
import { LinkedReportsAction } from "./visits/LinkedReportsAction";
import {
  ActionsCell,
  AppointmentCell,
  PlaceCell,
  StatusCell,
} from "./visits/VisitCells";
import { VisitDetailModal } from "./visits/VisitDetailModal";

type VisitSortKey = VisitsQueryRequest["sort"];
type ExcludedStatus = NonNullable<
  VisitsQueryRequest["excludeStatuses"]
>[number];

const pageSizes: readonly VisitsQueryRequest["pageSize"][] = [25, 50, 100];
const defaultPageSize = 50;
const descFirst: readonly VisitSortKey[] = ["prescribed"];
const sortKeys: readonly VisitSortKey[] = [
  "prescribed",
  "title",
  "status",
  "appointment",
  "place",
  "nre",
];

const statusKeys: Readonly<Record<ExcludedStatus, true>> = {
  booked: true,
  prescribed: true,
  unverified: true,
  handled: true,
  taken_in_charge: true,
  partially_done: true,
  suspended: true,
  done: true,
  done_after_cancel: true,
  cancelled: true,
  expired: true,
  other: true,
};

function isExcludedStatus(key: string): key is ExcludedStatus {
  return Object.hasOwn(statusKeys, key);
}

const focusedBg = "var(--mantine-primary-color-light)";
const rowPointer = { cursor: "pointer" };

export function VisitsView(): JSX.Element {
  const [search, setSearch] = useState("");
  const [excluded, setExcluded] = useState<ExcludedStatus[] | undefined>(
    undefined,
  );
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] =
    useState<VisitsQueryRequest["pageSize"]>(defaultPageSize);
  const [queryFocus, setQueryFocus] = useState<string | undefined>(undefined);
  const [handledFocus, setHandledFocus] = useState<string | undefined>(
    undefined,
  );
  const [handledRequest, setHandledRequest] = useState<string | undefined>(
    undefined,
  );
  const [detailNre, setDetailNre] = useState<string | undefined>(undefined);
  const [selectedNres, setSelectedNres] = useState<readonly string[]>([]);
  const [openReport, setOpenReport] = useState<ReportRefBody | undefined>(
    undefined,
  );
  const sort = useSort<VisitSortKey>("prescribed", "desc", descFirst);
  const requested = useRequestedVisit();

  const request: VisitsQueryRequest = {
    dir: sort.dir,
    page,
    pageSize,
    sort: sort.key,
    ...(search === "" ? {} : { search }),
    ...(excluded === undefined ? {} : { excludeStatuses: excluded }),
    ...(queryFocus === undefined ? {} : { focus: queryFocus }),
  };
  const query = useViewQuery("/api/visits/query", request);
  const { data, error, reload } = query;

  const routeFocus = useFocusTarget("visite", data?.focus?.found === true);

  if (routeFocus !== handledFocus) {
    setHandledFocus(routeFocus);
    if (routeFocus !== undefined) {
      setQueryFocus(routeFocus);
    }
  }

  if (requested === undefined && handledRequest !== undefined) {
    setHandledRequest(undefined);
  }
  if (requested !== undefined && requested !== handledRequest) {
    setHandledRequest(requested);
    setQueryFocus(requested);
    setDetailNre(requested);
  }

  if (
    data?.focus !== undefined &&
    queryFocus !== undefined &&
    data.focus.nre === queryFocus
  ) {
    setQueryFocus(undefined);
    setSearch(data.search);
    setExcluded(data.excludedStatuses.filter(isExcludedStatus));
    setPage(data.page);
  }

  useEffect(() => {
    if (requested !== undefined) {
      clearRequestedVisit();
    }
  }, [requested]);

  const refresh = async (scope: StartRefreshScope): Promise<void> => {
    try {
      const started = await startRefresh(scope);
      if (!started) {
        notifications.show({
          color: "red",
          message: "Impossibile avviare l'aggiornamento",
        });
        return;
      }
      reload();
    } catch {
      notifications.show({
        color: "red",
        message: "Impossibile avviare l'aggiornamento",
      });
    }
  };

  const rows = data?.rows ?? [];
  const selected = rows.filter((row) => selectedNres.includes(row.nre));

  const columns: DataTableColumn<VisitRowBody>[] = [
    {
      accessor: "prescribed",
      title: "Prescritta",
      sortable: true,
      noWrap: true,
      render: (row) => formatDate(row.prescribedOn),
    },
    {
      accessor: "title",
      title: "Prestazione",
      sortable: true,
      render: (row) => row.title,
    },
    {
      accessor: "status",
      title: "Stato",
      sortable: true,
      filtering: excluded !== undefined && excluded.length > 0,
      filter: (
        <StatusKeyFilter
          options={data?.statuses ?? []}
          onChange={(keys) => {
            setExcluded(keys.filter(isExcludedStatus));
            setPage(1);
          }}
        />
      ),
      render: (row) => <StatusCell row={row} />,
    },
    {
      accessor: "appointment",
      title: "Appuntamento",
      sortable: true,
      noWrap: true,
      width: 170,
      customCellAttributes: (row) => ({
        "aria-busy": row.appointmentLookup === "checking",
      }),
      render: (row) => (
        <AppointmentCell
          row={row}
          onOpen={() => {
            setDetailNre(row.nre);
          }}
        />
      ),
    },
    {
      accessor: "place",
      title: "Luogo",
      sortable: true,
      width: 96,
      textAlign: "center",
      render: (row) => <PlaceCell row={row} />,
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
      accessor: "actions",
      title: "Azioni",
      render: (row) => (
        <StopClick>
          <Group gap={2} wrap="nowrap">
            <ActionsCell row={row} />
            <LinkedReportsAction
              row={row}
              onOpenReport={setOpenReport}
              onOpenDetail={() => {
                setDetailNre(row.nre);
              }}
            />
          </Group>
        </StopClick>
      ),
    },
  ];

  const refreshing = data?.refreshing === true;
  const loadingFirst = data === undefined && error === undefined;
  const listsIssue =
    error === "unavailable" || data?.issues.lists === "unavailable";
  const listsLogin =
    error === "unauthorized" || data?.issues.lists === "login_required";
  const appointmentsIssue = data?.issues.appointments;

  return (
    <section aria-labelledby="visits-title" aria-busy={loadingFirst}>
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
            setPage(1);
          }}
        />
        <Text aria-live="polite">
          {`${String(data?.total ?? 0)} visite/esami`}
        </Text>
        {data === undefined || data.updated.label === "" ? null : (
          <Text size="sm" c="dimmed">
            {data.updated.label}
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
              setSelectedNres([]);
              void refresh({ nres: selected.map((row) => row.nre) });
            }}
          >
            {`Aggiorna selezionate (${String(selected.length)})`}
          </Button>
        )}
      </Group>
      {loadingFirst ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text>Carico visite ed esami…</Text>
        </Group>
      ) : null}
      {listsLogin ? (
        <Alert color="yellow" role="alert" mb="md">
          Accedi per vedere visite ed esami.
        </Alert>
      ) : null}
      {listsIssue ? (
        <Alert color="red" role="alert" mb="md">
          Il portale regionale non risponde. Riprova tra poco.
        </Alert>
      ) : null}
      {appointmentsIssue === "login_required" ? (
        <Alert color="yellow" role="alert" mb="md">
          Accedi per vedere gli appuntamenti.
        </Alert>
      ) : null}
      {error === "needs_gestione" ||
      appointmentsIssue === "gestione_required" ? (
        <Alert color="yellow" role="alert" mb="md">
          Per vedere date e orari serve l&apos;accesso a Gestione Prenotazioni:
          premi Accedi.
        </Alert>
      ) : null}
      {appointmentsIssue === "unavailable" ? (
        <Alert color="red" role="alert" mb="md">
          Appuntamenti non disponibili: il portale regionale non risponde.
        </Alert>
      ) : null}
      {data === undefined ? null : (
        <DataTable
          striped
          highlightOnHover
          withTableBorder
          {...tableColors}
          {...tableScrollProps}
          idAccessor="nre"
          records={rows}
          fetching={query.loading}
          totalRecords={data.total}
          page={data.page}
          onPageChange={setPage}
          recordsPerPage={pageSize}
          recordsPerPageOptions={[...pageSizes]}
          recordsPerPageLabel="Righe per pagina"
          paginationText={({ from, to, totalRecords }) =>
            `${String(from)}–${String(to)} di ${String(totalRecords)}`
          }
          onRecordsPerPageChange={(size) => {
            const next = pageSizes.find((item) => item === size);
            if (next !== undefined) {
              setPageSize(next);
              setPage(1);
            }
          }}
          selectedRecords={selected}
          onSelectedRecordsChange={(records) => {
            setSelectedNres(records.map((row) => row.nre));
          }}
          columns={columns}
          noRecordsText="Nessuna visita o esame in questa vista."
          sortStatus={{ columnAccessor: sort.key, direction: sort.dir }}
          onSortStatusChange={(status) => {
            const key = sortKeys.find((item) => item === status.columnAccessor);
            if (key !== undefined) {
              sort.toggle(key);
              setPage(1);
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
              row.nre.toUpperCase() === routeFocus ? "true" : undefined,
          })}
          rowStyle={(row) =>
            row.nre.toUpperCase() === routeFocus
              ? { ...rowPointer, backgroundColor: focusedBg }
              : rowPointer
          }
        />
      )}
      {detailNre === undefined ? null : (
        <VisitDetailModal
          nre={detailNre}
          onChanged={reload}
          onClose={() => {
            setDetailNre(undefined);
          }}
        />
      )}
      {openReport === undefined ? null : (
        <ReportDetailModal
          reportRef={openReport}
          onClose={() => {
            setOpenReport(undefined);
          }}
          onChanged={reload}
        />
      )}
    </section>
  );
}
