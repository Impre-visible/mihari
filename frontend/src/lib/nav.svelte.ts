// Which section is shown, and a job to open in it: shared so that a click on
// a notification, handled in Main, can open a transcript in Jobs.
export const nav = $state<{ section: string; jobId: number | null; wizard: boolean }>({
  section: "status",
  jobId: null,
  // Set to relaunch the setup assistant from the General page.
  wizard: false,
});
