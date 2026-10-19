// How GitLab writes a merge request or an issue in text: group/project!12 or group/project#42.
export function ref(project: string, targetType: string, iid: number) {
  return `${project}${targetType === "Issue" ? "#" : "!"}${iid}`;
}

// The same, without the group: where space is short and the project is known.
export function shortRef(project: string, targetType: string, iid: number) {
  return ref(project.split("/").pop() ?? project, targetType, iid);
}
