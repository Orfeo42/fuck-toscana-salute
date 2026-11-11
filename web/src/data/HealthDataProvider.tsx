import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from "react";

import {
  api,
  type AppointmentsBody,
  type PrescriptionsBody,
} from "../api/client";
import { useSession } from "../session/useSession";
import {
  HealthDataContext,
  type HealthData,
  type HealthError,
} from "./healthContext";
import { buildIndexes } from "./indexes";

interface Props {
  children: ReactNode;
}

interface Result<T> {
  requestId: string;
  data: T | undefined;
  error: HealthError | undefined;
}

function appointmentsError(status: number): HealthError {
  if (status === 401) {
    return "unauthorized";
  }
  if (status === 409) {
    return "needs_gestione";
  }
  return "unavailable";
}

export function HealthDataProvider({ children }: Props): JSX.Element {
  const { session } = useSession();
  const [appointmentReloads, setAppointmentReloads] = useState(0);
  const [prescriptionReloads, setPrescriptionReloads] = useState(0);
  const [appointmentResult, setAppointmentResult] = useState<
    Result<AppointmentsBody> | undefined
  >(undefined);
  const [prescriptionResult, setPrescriptionResult] = useState<
    Result<PrescriptionsBody> | undefined
  >(undefined);

  const cupState =
    session?.portals?.find((portal) => portal.portal === "cup")?.state ?? "";
  const gestioneReady = session?.gestioneReady ?? false;
  const portalKey =
    session?.portals?.map((p) => `${p.portal}:${p.state}`).join("|") ?? "";
  const appointmentId = `${cupState}|${String(gestioneReady)}|${String(appointmentReloads)}`;
  const prescriptionId = `${portalKey}#${String(prescriptionReloads)}`;

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data, response } = await api.GET("/api/appointments", {
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        if (data !== undefined) {
          setAppointmentResult({
            requestId: appointmentId,
            data,
            error: undefined,
          });
          return;
        }
        setAppointmentResult({
          requestId: appointmentId,
          data: undefined,
          error: appointmentsError(response.status),
        });
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setAppointmentResult({
          requestId: appointmentId,
          data: undefined,
          error: "unavailable",
        });
      }
    };
    void run();
    return () => {
      controller.abort();
    };
  }, [appointmentId]);

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data, response } = await api.GET("/api/prescriptions", {
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        if (data !== undefined) {
          setPrescriptionResult({
            requestId: prescriptionId,
            data,
            error: undefined,
          });
          return;
        }
        setPrescriptionResult({
          requestId: prescriptionId,
          data: undefined,
          error: response.status === 401 ? "unauthorized" : "unavailable",
        });
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setPrescriptionResult({
          requestId: prescriptionId,
          data: undefined,
          error: "unavailable",
        });
      }
    };
    void run();
    return () => {
      controller.abort();
    };
  }, [prescriptionId]);

  const reloadAppointments = useCallback((): void => {
    setAppointmentReloads((count) => count + 1);
  }, []);

  const reloadPrescriptions = useCallback((): void => {
    setPrescriptionReloads((count) => count + 1);
  }, []);

  const currentAppointments =
    appointmentResult?.requestId === appointmentId
      ? appointmentResult
      : undefined;
  const currentPrescriptions =
    prescriptionResult?.requestId === prescriptionId
      ? prescriptionResult
      : undefined;

  const appointmentData = currentAppointments?.data;
  const prescriptionData = currentPrescriptions?.data;
  const indexes = useMemo(
    () => buildIndexes(appointmentData, prescriptionData),
    [appointmentData, prescriptionData],
  );

  const value = useMemo<HealthData>(
    () => ({
      appointments: {
        data: appointmentData,
        error: currentAppointments?.error,
        loading: currentAppointments === undefined,
        reload: reloadAppointments,
      },
      prescriptions: {
        data: prescriptionData,
        error: currentPrescriptions?.error,
        loading: currentPrescriptions === undefined,
        reload: reloadPrescriptions,
      },
      ...indexes,
    }),
    [
      appointmentData,
      currentAppointments,
      reloadAppointments,
      prescriptionData,
      currentPrescriptions,
      reloadPrescriptions,
      indexes,
    ],
  );

  return <HealthDataContext value={value}>{children}</HealthDataContext>;
}
