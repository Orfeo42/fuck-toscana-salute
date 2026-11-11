import {
  Alert,
  Button,
  Group,
  Loader,
  SegmentedControl,
  Table,
  Text,
  TextInput,
} from "@mantine/core";
import { RefreshCw, Search } from "lucide-react";
import { useState } from "react";
import type { JSX } from "react";

import { CopyButton } from "../components/CopyButton";
import { PdfLink } from "../components/PdfLink";
import { SortHeader } from "../components/SortHeader";
import { StatusBadge } from "../components/StatusBadge";
import { useHealthData } from "../data/useHealthData";
import { formatDate } from "../lib/format";
import { useRoute } from "../lib/route";
import { useFocusTarget } from "../lib/useFocusTarget";
import { useSort } from "../lib/useSort";
import {
  countDrugs,
  drugRows,
  drugStatusLabel,
  drugStatusTone,
  filterDrugs,
  sortDrugs,
  type DrugFilter,
  type DrugRowData,
  type DrugSortKey,
} from "./drugs/filter";

const descFirst: readonly DrugSortKey[] = ["data"];

const filterOptions: readonly { value: DrugFilter; label: string }[] = [
  { value: "pickup", label: "Da ritirare" },
  { value: "all", label: "Tutte" },
];

const nowrap = { whiteSpace: "nowrap" } as const;
const focusedBg = "var(--mantine-primary-color-light)";

interface RowProps {
  row: DrugRowData;
}

function DrugRow({ row }: RowProps): JSX.Element {
  const { id, ref, tabIndex, focused } = useFocusTarget<HTMLTableRowElement>(
    "farmaci",
    row.nre,
  );
  return (
    <Table.Tr
      id={id}
      ref={ref}
      tabIndex={tabIndex}
      {...(focused ? { bg: focusedBg } : {})}
      data-focused={focused ? "true" : undefined}
    >
      <Table.Td style={nowrap}>{formatDate(row.fse.date)}</Table.Td>
      <Table.Td>{row.fse.description || "Farmaco"}</Table.Td>
      <Table.Td>
        <StatusBadge tone={drugStatusTone(row.fse.status)}>
          {drugStatusLabel(row.fse.status)}
        </StatusBadge>
      </Table.Td>
      <Table.Td style={nowrap}>
        <CopyButton value={row.nre} label="Copia NRE" />
      </Table.Td>
      <Table.Td>
        <Group gap={2} wrap="nowrap">
          <PdfLink documentRef={row.fse.documentRef} />
        </Group>
      </Table.Td>
    </Table.Tr>
  );
}

export function DrugsView(): JSX.Element {
  const { data: body, error, loading, reload } = useHealthData().prescriptions;
  const route = useRoute();
  const focus = route.tab === "farmaci" ? route.focus : undefined;
  const [handledFocus, setHandledFocus] = useState<string | undefined>(
    undefined,
  );
  const [filter, setFilter] = useState<DrugFilter>("pickup");
  const [search, setSearch] = useState("");
  const sort = useSort<DrugSortKey>("data", "desc", descFirst);

  const drugs = drugRows(body?.prescriptions ?? []);
  const counts = countDrugs(drugs, search);
  const visible = sortDrugs(
    filterDrugs(drugs, filter, search),
    sort.key,
    sort.dir,
  );

  if (body !== undefined && focus !== handledFocus) {
    setHandledFocus(focus);
    const exists = drugs.some((row) => row.nre.toUpperCase() === focus);
    const shown = visible.some((row) => row.nre.toUpperCase() === focus);
    if (exists && !shown) {
      setFilter("all");
      setSearch("");
    }
  }

  const segments = filterOptions.map((option) => ({
    value: option.value,
    label: `${option.label} (${String(counts[option.value])})`,
  }));

  const ready = !loading && error === undefined;

  return (
    <section aria-labelledby="drugs-title" aria-busy={loading}>
      <h2 id="drugs-title">Farmaci</h2>
      <Group align="flex-end" wrap="wrap" mb="md">
        <SegmentedControl
          aria-label="Mostra"
          data={segments}
          value={filter}
          onChange={(value) => {
            setFilter(value === "all" ? "all" : "pickup");
          }}
        />
        <TextInput
          label="Cerca"
          type="search"
          leftSection={<Search size={16} />}
          value={search}
          onChange={(event) => {
            setSearch(event.target.value);
          }}
        />
        <Text aria-live="polite">{`${String(visible.length)} farmaci`}</Text>
        <Button
          variant="light"
          leftSection={<RefreshCw size={16} />}
          onClick={reload}
        >
          Aggiorna
        </Button>
      </Group>
      {loading ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text>Carico i farmaci…</Text>
        </Group>
      ) : null}
      {!loading && error === "unauthorized" ? (
        <Alert color="yellow" role="alert">
          Accedi per vedere i farmaci.
        </Alert>
      ) : null}
      {!loading && error !== undefined && error !== "unauthorized" ? (
        <Alert color="red" role="alert">
          Il portale regionale non risponde. Riprova tra poco.
        </Alert>
      ) : null}
      {ready && visible.length === 0 ? (
        <Text c="dimmed">Nessun farmaco in questa vista.</Text>
      ) : null}
      {ready && visible.length > 0 ? (
        <Table.ScrollContainer minWidth={720}>
          <Table striped highlightOnHover stickyHeader verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <SortHeader label="Data" sortKey="data" sort={sort} />
                <SortHeader label="Farmaco" sortKey="farmaco" sort={sort} />
                <SortHeader label="Stato" sortKey="stato" sort={sort} />
                <SortHeader label="NRE" sortKey="nre" sort={sort} />
                <Table.Th>Azioni</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {visible.map((row) => (
                <DrugRow key={row.nre} row={row} />
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      ) : null}
    </section>
  );
}
