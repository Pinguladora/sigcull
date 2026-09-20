// Ambient declarations for the Starlight virtual modules the vendored components
// import. Starlight injects these modules at build time but does not ship their
// type declarations to consumer projects, so astro check cannot resolve them in
// our overrides (Search.astro). Shapes mirror Starlight's vite-virtual-modules.
declare module "virtual:starlight/project-context" {
  const project: {
    build: { format: "file" | "directory" | "preserve" };
    root: string;
    srcDir: string;
    trailingSlash: "always" | "never" | "ignore";
  };
  export default project;
}

declare module "virtual:starlight/pagefind-config" {
  export const pagefindUserConfig: Record<string, unknown>;
}
