// Which section is shown, and the job open in it as its own page: shared so
// that a click on a notification, handled in Main, can open a job.
export const nav = $state<{ section: string; jobId: number | null; wizard: boolean }>({
  section: "status",
  jobId: null,
  // Set to relaunch the setup assistant from the General page.
  wizard: false,
});
