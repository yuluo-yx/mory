# 给思考留一点空间

欢迎使用 **Mory**，这里是一个存放本地文稿的空间。这份文档是一份可操作的导览：可以直接编辑示例，切换到 **源码** 查看写法，或使用 **分屏** 同时查看源码和预览。

你的文稿会保存在这台 Mac 上。需要重新查看这份导览时，可以打开 **帮助 → 打开欢迎指南**。已经编辑过的欢迎文档会被保留。

## 从这里开始

- [x] 打开写作空间
- [ ] 使用 **⌘N** 创建文稿
- [ ] 试用 **写作**、**源码**、**分屏** 和 **阅读** 模式
- [ ] 将鼠标移到标题上，查看标题层级并折叠章节
- [ ] 打开 **设置 → 编辑器**，调整字号、目录字号、页面宽度和页面位置

可以使用文档大纲在章节之间移动，也可以把大纲放在文档左侧或右侧。专注模式会隐藏周围的导航；全屏和页面位置设置可以帮助你找到舒适的写作区域。

## 日常 Markdown

可以使用 **粗体**、*斜体*、***粗斜体***、~~删除线~~ 和 `行内代码`。 [网页链接](https://github.com/yuluo-yx/mory) 可以直接点击。也可以 [跳转到 Docusaurus 示例](#docusaurus-examples)。

```markdown
**粗体**  *斜体*  ***粗斜体***  ~~删除线~~  `行内代码`
[网页链接](https://github.com/yuluo-yx/mory)
```

段落之间使用空行分隔。在行末添加反斜杠可以插入硬换行。\
这一行仍然属于同一个段落。

### 三级标题

在 `#` 到 `######` 后加空格，可以分别创建 Markdown 的一到六级标题。这份指南已经包含一、二级标题。

#### 四级标题

##### 五级标题

###### 六级标题

<details><summary>更高的原生标题层级</summary>

Mory 还通过结构化 XML 支持七到九级标题：

<h7>七级标题</h7>
<h8>八级标题</h8>
<h9>九级标题</h9>

```xml
<h7>七级标题</h7>
<h8>八级标题</h8>
<h9>九级标题</h9>
```

</details>

### 列表和任务

- 无序列表项
- 另一个列表项
  - 嵌套列表项
  - 第二个嵌套列表项

1. 收集一个想法。
2. 为它写一个合适的标题。
3. 把下一步变成一个任务。

- [ ] 未完成的任务 — 点击复选框即可完成
- [x] 已完成的任务

### 引用和分隔线

> 好的写作始于一个安静的空间和一张空白的页面。
>
> 引用可以包含多个段落。

下面这条分隔线使用 `---` 写成：

---

## 代码和技术写作

围栏代码块支持语法高亮、语言选择、自动换行和复制。代码在写作模式下仍然可以编辑。

```go title="hello.go" {3} showLineNumbers
package main

func main() {
    println("Hello from Mory")
}
```

上面的围栏元数据设置了文件名、突出显示第 3 行并显示行号。也支持 Docusaurus 的高亮注释；这些注释仍然可见并可以编辑：

```javascript
// highlight-next-line
const message = 'A small idea worth keeping';
// highlight-start
console.log(message);
console.log('Keep writing.');
// highlight-end
```

### 表格

| 功能 | 试用位置 | 格式 |
| :--- | :---: | ---: |
| 行内格式 | 本指南 | Markdown |
| 表格编辑 | 点击单元格 | 原生 |
| 源码保留 | 切换模式 | 无损 |

使用表格工具可以添加或删除行列、选择单元格、合并或拆分单元格、调整列宽以及设置单元格颜色。简单表格会导出为 Markdown；合并单元格需要使用嵌入式 XML。

<details><summary>合并单元格的表格及其源码</summary>

<table><tbody><tr><th colspan="2"><p>共享标题</p></th></tr><tr><td><p>左侧单元格</p></td><td background-color="#e8f4ec"><p>带颜色的右侧单元格</p></td></tr></tbody></table>

```xml
<table><tbody>
  <tr><th colspan="2"><p>共享标题</p></th></tr>
  <tr><td><p>左侧单元格</p></td><td><p>右侧单元格</p></td></tr>
</tbody></table>
```

</details>

### 公式

行内数学公式使用美元符号：$E = mc^2$。块级公式使用一对双美元符号：

$$
\int_0^1 x^2\,dx = \frac{1}{3}
$$

$$
\begin{bmatrix}1 & 2 \\ 3 & 4\end{bmatrix}
$$

选择 **插入 → 数学公式** 创建块级公式，或选择 **行内公式** 创建行内对象。选中公式后可以编辑 LaTeX。分数、矩阵、积分以及 KaTeX 支持的其他写法都会在本地渲染。

## 提示框和 Docusaurus {#docusaurus-examples}

使用带标记的引用可以创建 GitHub 风格的提示框：

> [!NOTE]
> 这是一个使用 `> [!NOTE]` 写成的提示。

:::tip 试用原生预览
选择 **设置 → 主题 → Docusaurus**，体验文档风格。同一份源码也适用于 Craft、飞书、Notion 和 Obsidian。
:::

:::info 保留源码
Docusaurus YAML front matter 会在源码模式中保留，不会显示为正文。ID、标题、标签、slug 和其他字段都会保持原始写法。
:::

:::warning 适用范围
Mory 可以预览静态文档功能。React 组件、JavaScript 表达式、站点专用别名和自定义构建插件仍然需要原站点的运行环境。Mermaid 围栏会继续作为代码显示。
:::

<details><summary>其他提示框类型和嵌套内容</summary>

:::note 普通提示
可以使用 `:::note`、`:::tip`、`:::info`、`:::warning`、`:::danger` 或 `:::caution`。

:::tip 嵌套提示
提示框可以包含段落、列表、代码和其他提示框。
:::

:::

:::danger 危险提示示例
这里只是视觉示例，不会执行任何操作。
:::

:::caution 注意事项示例
选择一个能够说明当前情况的标题。
:::

</details>

### 可折叠内容和标签页

点击摘要可以展开或折叠内容。点击下方标签页可以切换显示的说明。这些操作不会改写文档。

<Tabs groupId="guide-platform" defaultValue="mac">
<TabItem value="mac" label="macOS" default>

按 **⌘N** 创建文稿，按 **⌘S** 保存。使用 **⌘,** 打开设置。

</TabItem>
<TabItem value="source" label="源码语法">

```mdx
<details><summary>阅读更多</summary>

这里可以放嵌套的 **Markdown**。

</details>

<Tabs defaultValue="first">
<TabItem value="first" label="第一个">第一个标签页。</TabItem>
<TabItem value="second" label="第二个">第二个标签页。</TabItem>
</Tabs>
```

</TabItem>
</Tabs>

支持静态的 `defaultValue`、`default` 和 `groupId`。共享组 ID 的标签页会在同一份文档中共享选择状态。HTML 和 PDF 导出会包含所有标签页。

<details><summary>Front matter 和显式标题 ID</summary>

把 YAML front matter 放在文档最开头：

```yaml
---
id: writing-guide
title: Writing guide
slug: /writing
sidebar_position: 1
tags: [writing, local]
---
```

使用 `## 稳定标题 {#stable-heading}` 指定标题锚点，再通过 `[跳转](#stable-heading)` 链接到它。锚点标记不会显示在渲染后的标题中。

</details>

## 图片、附件和文档详情

![Mory 插图](https://raw.githubusercontent.com/yuluo-yx/mory/main/resources/icon.png)

上面的图片托管在 Mory 的 GitHub 仓库中，联网时会加载。使用 **插入 → 图片或附件……** 插入自己的图片或文件。选中图片后可以设置说明、尺寸、对齐方式和裁剪位置。文件附件可以使用 macOS 的快速查看预览。

```markdown
![有用的说明](https://raw.githubusercontent.com/yuluo-yx/mory/main/resources/icon.png)
```

使用 **添加图标**、**添加封面** 或 **文档操作** 自定义文档。封面位置和正文位置独立于段落文本对齐方式。本地资源会在导入时复制；打开本地文件则会直接编辑原文件。

## Markdown 中的原生块

有些功能需要的结构超出了标准 Markdown。Mory 会把这些块以嵌入式 XML 的形式保存在 `.md` 文件中，因此编辑和导出时仍能保留属性。其他 Markdown 渲染器可能无法以相同方式显示这些内容。

### 颜色、下划线和对齐

<p><u>带下划线的文字</u>、<span text-color="blue">蓝色文字</span> 和 <span background-color="light-yellow">高亮文字</span>。</p>

<p align="center">居中的段落。</p>

<p align="right">右对齐的段落。</p>

```xml
<p><u>下划线</u>和<span text-color="blue">颜色</span></p>
<p><span background-color="light-yellow">高亮</span></p>
<p align="center">居中文本</p>
```

### 分栏

<grid><column width-ratio="1"><h3>收集</h3><p>把零散想法、问题和参考资料放在这里。</p></column><column width-ratio="1"><h3>展开</h3><p>把有用的内容整理成段落或计划。</p></column></grid>

使用 **插入 → 分栏**，然后调整分栏工具。源码如下：

```xml
<grid>
  <column width-ratio="1"><p>左栏</p></column>
  <column width-ratio="1"><p>右栏</p></column>
</grid>
```

### 倒计时

<countdown duration="300"/>

这个五分钟计时器只有在按下 **开始 / 重置** 后才会启动。可以用它进行一段短时间的写作。**停止** 会将计时器恢复到初始时长。

### 提醒、按钮和文档引用

选择 **插入 → 提醒**、**按钮** 或 **文档链接**，添加可编辑的行内对象。可以在 Mory 中配置它们的属性；创建文档引用时，选择一个已有文稿。提醒会在 Mory 运行期间检查。

```xml
<p><time date="2030-01-01" time="09:00"/> 一个提醒</p>
<p><button text="项目网站" href="https://github.com/yuluo-yx/mory"/></p>
```

这些只是源码示例，因此阅读指南不会安排提醒。使用 **插入 → 子文稿** 显示父文稿的子文稿；点击侧边栏中父文稿旁的加号创建子文稿。

## 编辑、整理和保存工作

| 任务 | 操作方式 |
| --- | --- |
| 插入块 | 在新段落中输入 `/`，使用 **+ 插入**，或按 **⌘P** |
| 选择或移动块 | 使用块的操作柄和块操作菜单 |
| 查找和替换 | 使用 **⌘F** 查找；使用 **⌘⌥F** 替换 |
| 搜索文库 | **⌘J** |
| 修改快捷键 | **设置 → 键盘快捷键** |
| 调整页面和大纲 | **设置 → 编辑器** |
| 编辑文档主题 | **设置 → 主题 → 编辑 CSS** |
| 使用 Mory 打开 Markdown 文件 | **设置 → 通用 → 默认应用** |
| 导入副本 | **文件 → 导入……** 或导入文件夹 |
| 编辑原文件 | **文件 → 打开本地文件** |
| 导出或打印 | 导出 Markdown、DocxXML、HTML 或 PDF；使用 **文件 → 打印** |
| 整理文稿 | 文件夹、父文稿、收藏、置顶、手动排序、列表或网格视图 |
| 恢复工作 | 回收站、文档详情中的历史版本，以及保存失败后的恢复内容 |

编辑开始后会自动保存；按 **⌘S** 可以立即保存。大型纯文本会延迟到保存边界再进行完整序列化。如果检测到冲突写入，Mory 会报告冲突，不会静默覆盖更新的文件。

## 一个小型 JavaScript 工作台

打开 **插件**，可以清理行尾空格、统计字数或生成目录。本地插件导出一个 `transform` 函数：

```javascript title="replace-todos.js"
export function transform({ markdown }) {
  return {
    markdown: markdown.replace(/TODO:/g, '- [ ]'),
    message: 'Converted TODO markers',
  };
}
```

将文件放入文库的 `plugins/` 文件夹，然后选择 **重新加载插件**。插件会在时间和内存限制下本地运行，不能使用文件、网络、环境变量和 Node.js API。远程集成不属于此插件接口的一部分。

现在可以用自己的文字替换这份指南，也可以把它留在身边作为参考。
