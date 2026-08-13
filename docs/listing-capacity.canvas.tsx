import {
  BarChart,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Divider,
  Grid,
  H1,
  H2,
  LineChart,
  Stack,
  Stat,
  Table,
  Text,
  UsageBar,
} from "cursor/canvas";

const HANDLE_MS = [3.1, 34.7, 169.9, 686.2, 7341, 29485];
const HANDLE_LOTS = ["50", "200", "500", "1k", "2.5k", "5k"];

export default function ListingCapacity() {
  return (
    <Stack gap={24}>
      <Stack gap={8}>
        <H1>Ёмкость листинга — order-bot</H1>
        <Text tone="secondary">
          Нагрузочный прогон на этой машине: engine.Handle по трём книгам
          (MRKT + Portals + Getgems) и HTTP-бюджет по живым rate-gate.
          Интервал опроса 90с. Источник: go test ./internal/engine -run
          TestLoad, 13 авг 2026. Копия в репозитории:
          docs/listing-capacity.canvas.tsx
        </Text>
      </Stack>

      <Callout tone="success" title="Оптимум для paper-снайпа">
        5–8 слотов вида коллекция + модель (backdrop по желанию), до ~200
        живых лотов на маркет после фильтра. Тик HTTP 15–30с, Handle меньше
        50мс. Голая коллекция на 500+ asks уже съедает окно Portals/MRKT.
      </Callout>

      <Grid columns={3} gap={16}>
        <Stat value="5–8" label="Watch-слоты" tone="success" />
        <Stat value="3–5 × 1–2" label="Коллекции × модели" tone="success" />
        <Stat value="≤500" label="Потолок до лагов Handle" tone="warning" />
      </Grid>

      <Stack gap={8}>
        <H2>Бюджет poll: 8 слотов × 200 лотов</H2>
        <Text tone="secondary" size="small">
          Секунды последовательного List внутри одного poll 90с (воркеры
          параллельны — лимитирует самый медленный маркет). Запас 70% = 63с.
        </Text>
        <UsageBar
          total={90}
          topLeftLabel="8 слотов × 200 лотов, середина gate"
          topRightLabel="poll 90с"
          segments={[
            { id: "mrkt", value: 24.3, color: "blue" },
            { id: "gap", value: 65.7, color: "gray" },
          ]}
        />
        <Text size="small" tone="tertiary">
          MRKT ~24с · Portals ~29с · Getgems ~18с. Все трое укладываются;
          лимитер — Portals (страница по 20).
        </Text>
      </Stack>

      <Divider />

      <Stack gap={8}>
        <H2>Время Handle vs размер книги (до индекса)</H2>
        <Text tone="secondary" size="small">
          Один MODEL_BG на трёх книгах, без спреда. Старый evalLot сканировал
          каждый quote-лот на каждый buy — O(n²) плюс giftid.Fold. Память не
          предел (мегабайты). После индекса askByKey lookup O(1).
        </Text>
        <LineChart
          categories={HANDLE_LOTS}
          series={[{ name: "Handle (мс)", data: HANDLE_MS, tone: "danger" }]}
          height={220}
          fill
          valueSuffix=" мс"
          referenceLines={[{ value: 100, label: "100мс", tone: "warning" }]}
        />
        <Text size="small" tone="tertiary">
          50–1000 — этот прогон; 2.5k = 7.3с и 5k = 29.5с с первого прохода
          (10k не уложился в 2 мин). Это и есть причина индекса.
        </Text>
      </Stack>

      <Table
        headers={["Лотов / маркет", "Handle", "Влезает в 90с?", "Заметка"]}
        columnAlign={["right", "right", "left", "left"]}
        striped
        rows={[
          ["50", "3 мс", "да", "Типичный слот с моделью"],
          ["200", "35 мс", "да", "Комфортно"],
          ["500", "170 мс", "да", "Норм, если слотов мало"],
          ["1 000", "0.7 с", "CPU ок, HTTP туго", "Portals ~15с на слот"],
          ["2 500", "7.3 с", "нет", "Handle сам по себе тормозит тик"],
          ["5 000", "29.5 с", "нет", "Не влезает в poll; 10k timeout"],
        ]}
        rowTone={[
          "success",
          "success",
          "warning",
          "warning",
          "danger",
          "danger",
        ]}
      />

      <Divider />

      <Stack gap={8}>
        <H2>Макс. слотов в HTTP-бюджете 63с</H2>
        <Text tone="secondary" size="small">
          Слот = полный стакан на столько лотов. Gate: MRKT 275мс/стр.20 ·
          Portals 300мс/стр.20 · Getgems 500мс/стр.100. Пауза между слотами
          325мс.
        </Text>
        <BarChart
          categories={["20", "50", "100", "200", "500", "1k", "2k", "5k"]}
          series={[
            { name: "MRKT", data: [105, 54, 37, 20, 8, 4, 2, 0] },
            { name: "Portals", data: [68, 41, 29, 17, 7, 4, 2, 0] },
            { name: "Getgems", data: [34, 34, 34, 27, 16, 9, 5, 2] },
          ]}
          height={240}
          showValues={false}
          referenceLines={[{ value: 8, label: "8 слотов", tone: "success" }]}
        />
        <Text size="small" tone="tertiary">
          Категории = лотов на слот. Getgems дешевле за лот (страница 100),
          но API не фильтрует Model — качается вся коллекция.
        </Text>
      </Stack>

      <Card>
        <CardHeader trailing="держать ниже 63с">Длительность тика, секунды</CardHeader>
        <CardBody>
          <Table
            framed={false}
            headers={["Слоты", "Лотов/слот", "MRKT", "Portals", "Getgems"]}
            columnAlign={["right", "right", "right", "right", "right"]}
            striped
            rows={[
              ["5", "50", "5.4", "7.3", "8.8"],
              ["5", "200", "15", "18", "11"],
              ["8", "50", "8.9", "12", "14"],
              ["8", "200", "24", "29", "18"],
              ["8", "500", "57", "65", "30"],
              ["12", "200", "37", "43", "28"],
            ]}
            rowTone={[
              "success",
              "success",
              "success",
              "success",
              "danger",
              "warning",
            ]}
          />
        </CardBody>
      </Card>

      <Divider />

      <Stack gap={8}>
        <H2>Что делают похожие парсеры</H2>
        <Text>
          OpenSea / Reservoir: trait-фильтр на сервере (Model=Detective) и
          отдельный best-listings. Getgems Read API этого не даёт — только
          cursor+limit по коллекции. Значит паттерн тот же, что у gift-bot
          Portals listCache: один List коллекции на тик, модели режем у себя.
        </Text>
        <Table
          headers={["Площадка", "Фильтр модели", "Что делаем"]}
          striped
          rows={[
            ["MRKT / Portals", "в API", "как есть: маленькие страницы"],
            [
              "Getgems",
              "нет в API",
              "кэш стакана коллекции 20с, модели с кэша",
            ],
            [
              "Engine",
              "—",
              "индекс min-ask по MODEL_BG, не полный скан книги",
            ],
          ]}
        />
      </Stack>

      <Callout tone="info" title="Что уже поправлено в коде">
        Getgems больше не качает одну коллекцию заново на каждую модель в
        пределах 20с (как TTL у Portals). Engine больше не сканирует всю
        книгу на каждый лот: после снимка строится индекс cheapest ask +
        collection floor.
      </Callout>
    </Stack>
  );
}
