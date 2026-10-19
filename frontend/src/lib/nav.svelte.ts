// Which section is shown, and the job selected in Activité: shared so that a
// click on a notification, handled in Main, can open a job.
export const nav = $state<{ section: string; jobId: number | null; wizard: boolean }>({
  section: "activity",
  jobId: null,
  // Set to relaunch the setup assistant from the General page.
  wizard: false,
});
