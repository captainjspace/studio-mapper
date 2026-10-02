// Starting point for the band app (rollingblackoutapp): renders the tracks from
// `studio-map routing --json` (RoutingDoc v1) as a table.
type Track = {
  label: string;
  musician?: string;
  source?: string;
  mic?: string;
  preamp?: string;
  interface?: string;
  hostIn?: number;
  stem?: boolean;
  channelPreset?: string;
};

const columns: [string, (t: Track) => string | number][] = [
  ["In", (t) => (t.stem ? "Stem" : (t.hostIn ?? ""))],
  ["Track", (t) => t.label],
  ["Who", (t) => t.musician ?? ""],
  ["Source", (t) => t.source ?? ""],
  ["Mic", (t) => t.mic ?? ""],
  ["Preamp", (t) => t.preamp ?? ""],
  ["Patch", (t) => t.interface ?? ""],
  ["Channel preset", (t) => t.channelPreset ?? ""],
];

export default function TrackTable({ tracks }: { tracks: Track[] }) {
  if (tracks.length === 0) {
    return <p>No tracks.</p>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead>
          <tr>
            {columns.map(([h]) => (
              <th key={h} className="px-3 py-2">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {tracks.map((t) => (
            <tr key={t.label} className="border-t">
              {columns.map(([h, f]) => (
                <td key={h} className="px-3 py-2">
                  {f(t)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
