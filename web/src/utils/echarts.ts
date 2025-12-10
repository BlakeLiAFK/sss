// ECharts 按需引入
// 只引入使用到的组件，大幅减少打包体积

import * as echarts from 'echarts/core'

// 引入饼图
import { PieChart } from 'echarts/charts'

// 引入提示框、标题、图例等组件
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent
} from 'echarts/components'

// 引入 Canvas 渲染器
import { CanvasRenderer } from 'echarts/renderers'

// 注册必须的组件
echarts.use([
  PieChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  CanvasRenderer
])

export default echarts
export type { ECharts } from 'echarts/core'
