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
import { useState } from "react";
import type { JSX, KeyboardEvent } from "react";

import type { DrugRowBody, DrugsQueryRequest } from "../api/client";
import { CopyButton } from "../components/CopyButton";
import { PdfLink } from "../components/PdfLink";
import { StatusBadge } from "../components/StatusBadge";
import { StatusKeyFilter } from "../components/StatusKeyFilter";
import { StopClick } from "../components/StopClick";
import { tableScrollProps } from "../components/tableScroll";
import { startRefresh } from "../data/refresh";
import { formatDate } from "../lib/format";
import { focusId, useFocusTarget } from "../lib/useFocusTarget";
import { useRoute } from "../lib/route";
import { useSort } from "../lib/useSort";
import { useViewQuery } from "../lib/useViewQuery";
import { tableColors } from "../theme";
import { DrugDetailModal } from "./drugs/DrugDetailModal";

type SortKey = DrugsQueryRequest["sort"];
type PageSize = DrugsQueryRequest["pageSize"];
type ExcludedKey = NonNullable<DrugsQueryRequest["excludeStatuses"]>[number];

const sortKeys: readonly SortKey[] = ["date", "name", "status", "nre"];
const descFirst: readonly SortKey[] = ["date"];
const pageSizes: readonly PageSize[] = [25, 50, 100];
const focusedBg = "var(--mantine-primary-color-light)";
const rowPointer = { cursor: "pointer" };

export function DrugsView(): JSX.Element {
  const [search, setSearch] = useState("");
  const [excluded, setExcluded] = useState<string[] | undefined>(undefined);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<PageSize>(50);
  const [detailNre, setDetailNre] = useState<string | undefined>(undefined);
  const [focusDone, setFocusDone] = useState<string | undefined>(undefined);
  const [starting, setStarting] = useState(false);
  const sort = useSort<SortKey>("date", "desc", descFirst);
  const current = useRoute();
  const route = current.tab === "farmaci" ? current.focus : undefined;
  const focus = route === focusDone ? undefined : route;

  const request: DrugsQueryRequest = {
    search,
    sort: sort.key,
    dir: sort.dir,
    page,
    pageSize,
    ...(excluded === undefined
      ? {}
      : { excludeStatuses: excluded as ExcludedKey[] }),
    ...(focus === undefined ? {} : { focus }),
  };
  const { data, error, loading, reload } = useViewQuery(
    "/api/drugs/query",
    request,
  );
  useFocusTarget("farmaci", data !== undefined && focus === undefined);

  if (
    focus !== undefined &&
    data?.focus?.nre.toUpperCase() === focus.toUpperCase()
  ) {
    setFocusDone(route);
    if (data.focus.found) {
      setPage(data.page);
      setExcluded(data.excludedStatuses);
      setSearch(data.search);
    }
  }

  const refreshLists = async (): Promise<void> => {
    setStarting(true);
    try {
      const ok = await startRefresh({ lists: true });
      if (!ok) {
        notifications.show({
          color: "red",
          message: "Aggiornamento non riuscito",
        });
        return;
      }
      reload();
    } catch {
      notifications.show({
        color: "red",
        message: "Aggiornamento non riuscito",
      });
    } finally {
      setStarting(false);
    }
  };

  const columns: DataTableColumn<DrugRowBody>[] = [
    {
      accessor: "date",
      title: "Data",
      sortable: true,
      noWrap: true,
      render: (row) => formatDate(row.date),
    },
    {
      accessor: "name",
      title: "Farmaco",
      sortable: true,
      render: (row) => row.title,
    },
    {
      accessor: "status",
      title: "Stato",
      sortable: true,
      filtering: (data?.excludedStatuses.length ?? 0) > 0,
      filter: (
        <StatusKeyFilter
          options={data?.statuses ?? []}
          onChange={(keys) => {
            setExcluded(keys);
            setPage(1);
          }}
        />
      ),
      render: (row) => (
        <StatusBadge tone={row.status.tone}>{row.status.label}</StatusBadge>
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
          <Group gap={2} wrap="nowrap">
            <PdfLink documentRef={row.documentRef} />
          </Group>
        </StopClick>
      ),
    },
  ];

  const unauthorized =
    error === "unauthorized" || data?.issue === "login_required";
  const unavailable =
    (error !== undefined && error !== "unauthorized") ||
    data?.issue === "unavailable";
  const showTable = data !== undefined && error === undefined;
  const refreshing = starting || data?.refreshing === true;

  return (
    <section aria-labelledby="drugs-title" aria-busy={loading}>
      <Title order={2} id="drugs-title">
        Farmaci
      </Title>
      <Group align="flex-end" wrap="wrap" mb="md">
        <TextInput
          label="Cerca"
          type="search"
          leftSection={<Search size={16} />}
          value={search}
          onChange={(event) => {
            setSearch(event.target.value);
            setPage(1);
          }}
        />
        {data === undefined ? null : (
          <Text aria-live="polite">{`${String(data.total)} farmaci`}</Text>
        )}
        {data === undefined || data.updated.label === "" ? null : (
          <Text size="sm" c="dimmed">
            {data.updated.label}
          </Text>
        )}
        <Button
          variant="light"
          loading={refreshing}
          leftSection={<RefreshCw size={16} />}
          onClick={() => {
            void refreshLists();
          }}
        >
          Aggiorna
        </Button>
      </Group>
      {loading && data === undefined ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text>Carico i farmaci…</Text>
        </Group>
      ) : null}
      {unauthorized ? (
        <Alert color="yellow" role="alert">
          Accedi per vedere i farmaci.
        </Alert>
      ) : null}
      {unavailable && !unauthorized ? (
        <Alert color="red" role="alert">
          Il portale regionale non risponde. Riprova tra poco.
        </Alert>
      ) : null}
      {showTable ? (
        <DataTable
          striped
          highlightOnHover
          withTableBorder
          {...tableColors}
          {...tableScrollProps}
          idAccessor="nre"
          fetching={loading}
          records={data.rows}
          columns={columns}
          totalRecords={data.total}
          page={data.page}
          onPageChange={setPage}
          recordsPerPage={data.pageSize}
          recordsPerPageOptions={[...pageSizes]}
          onRecordsPerPageChange={(size) => {
            const next = pageSizes.find((item) => item === size);
            if (next !== undefined) {
              setPageSize(next);
              setPage(1);
            }
          }}
          noRecordsText="Nessun farmaco in questa vista."
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
            id: focusId("farmaci", row.nre),
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
              row.nre.toUpperCase() === route?.toUpperCase()
                ? "true"
                : undefined,
          })}
          rowStyle={(row) =>
            row.nre.toUpperCase() === route?.toUpperCase()
              ? { ...rowPointer, backgroundColor: focusedBg }
              : rowPointer
          }
        />
      ) : null}
      {detailNre === undefined ? null : (
        <DrugDetailModal
          nre={detailNre}
          onClose={() => {
            setDetailNre(undefined);
          }}
        />
      )}
    </section>
  );
}
