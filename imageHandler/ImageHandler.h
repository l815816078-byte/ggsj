#ifndef OPENCV430_IMAGEHANDLER_H
#define OPENCV430_IMAGEHANDLER_H

#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

/**
 * 释放启用本地日志
 * @param path 日志路径 可为""
 * @param flag  是否启用
 * @return 日志路径 （返回值需用 DeleteCharArr 释放)
 */
const char *UseLocalLog(char *path, bool flag);


// 创建新的vector<Point>并返回句柄
long PointVector();

// 释放vector<Point>内存
void Delete_PointVector(long _pointVector);

// 添加Point元素
void PointVector_Add(long _pointVector, int x, int y);

// 插入Point元素到指定索引
void PointVector_Insert(long _pointVector, int index, int x, int y);
// 删除指定索引的Point元素
void PointVector_RemoveAt(long _pointVector, int index);

// 删除第一个匹配的Point元素
bool PointVector_Remove(long _pointVector, int x, int y);

// 获取指定索引的Point元素
bool PointVector_Get(long _pointVector, int index, int *outX, int *outY);

// 获取向量大小
int PointVector_Size(long _pointVector);


// 创建新的二维点向量并返回句柄
long PointVector2D();

// 释放二维点向量内存
void Delete_PointVector2D(long _pointVector2D);

// 向二维向量添加一个点向量（内部向量）
void PointVector2D_AddVector(long _pointVector2D, long _pointVector);

// 向二维向量的指定索引位置插入一个点向量
void PointVector2D_InsertVector(long _pointVector2D, int index, long _pointVector);

// 删除二维向量中指定索引的内部向量
void PointVector2D_RemoveVectorAt(long _pointVector2D, int index);

// 获取二维向量的大小（包含多少个内部向量）
int PointVector2D_Size(long _pointVector2D);

// 获取二维向量中指定索引的内部向量（返回新的向量句柄，需要单独释放）
long PointVector2D_GetVector(long _pointVector2D, int index);

// 清空二维向量中的所有元素
void PointVector2D_Clear(long _pointVector2D);


// 创建一个新的 int 向量并返回其句柄（内部为 std::vector<int>* 转换为 long）
long IntVector();

// 释放 int 向量的内存（需传入有效的句柄）
void Delete_IntVector(long _intVector);

// 向向量添加元素
void IntVector_Add(long _intVector, int value);

// 在指定索引插入元素（索引超出范围则忽略）
void IntVector_Insert(long _intVector, int index, int value);

// 删除指定索引的元素（索引无效则忽略）
void IntVector_RemoveAt(long _intVector, int index);

// 删除第一个匹配的元素（返回是否删除成功）
bool IntVector_Remove(long _intVector, int value);

// 检查向量是否包含指定元素
bool IntVector_Contains(long _intVector, int value);

// 获取向量中元素的数量
int IntVector_Size(long _intVector);

// 获取指定索引的元素（索引无效返回 0，可根据需求调整默认值）
int IntVector_Get(long _intVector, int index);

// 修改指定索引的元素（索引无效则忽略）
void IntVector_Set(long _intVector, int index, int value);

// 清空向量中的所有元素
void IntVector_Clear(long _intVector);

// 获取向量中指定元素的第一个索引（不存在返回 - 1）
int IntVector_IndexOf(long _intVector, int value);


// 创建新的二维整数向量并返回句柄
long IntVector2D();

// 释放二维整数向量内存
void Delete_IntVector2D(long _intVector2D);

// 向二维向量添加一个整数向量（内部向量）
void IntVector2D_AddVector(long _intVector2D, long _intVector);

// 向二维向量的指定索引位置插入一个整数向量
void IntVector2D_InsertVector(long _intVector2D, int index, long _intVector);

// 删除二维向量中指定索引的内部向量
void IntVector2D_RemoveVectorAt(long _intVector2D, int index);

// 获取二维向量的大小（包含多少个内部向量）
int IntVector2D_Size(long _intVector2D);

// 获取二维向量中指定索引的内部向量（返回新的向量句柄，需要单独释放）
long IntVector2D_GetVector(long _intVector2D, int index);

// 清空二维向量中的所有元素
void IntVector2D_Clear(long _intVector2D);

// 向二维向量的指定内部向量添加一个整数元素
void IntVector2D_AddElement(long _intVector2D, int vecIndex, int value);

// 获取二维向量中指定位置的元素值
int IntVector2D_GetElement(long _intVector2D, int vecIndex, int elemIndex);

// 获取二维向量中指定内部向量的大小
int IntVector2D_GetVectorSize(long _intVector2D, int vecIndex);




//vector<string>

long StringVector();

void Delete_StringVector(long _stringVector);

// 向向量添加元素
void StringVector_Add(long _stringVector, const char *str);

// 从向量中删除指定索引的元素
void StringVector_RemoveAt(long _stringVector, int index);

// 从向量中删除指定字符串
bool StringVector_Remove(long _stringVector, const char *str);

// 检查向量是否包含指定字符串
bool StringVector_Contains(long _stringVector, const char *str);

// 获取向量中元素的数量
int StringVector_Size(long _stringVector);

// 获取指定索引的元素（返回字符串指针，需注意生命周期）
const char *StringVector_Get(long _stringVector, int index);

// 清空向量中的所有元素
void StringVector_Clear(long _stringVector);



//vector<long>

// 创建一个新的 long 向量并返回其指针（作为 long 类型）
long LongVector();

// 释放 long 向量的内存
void Delete_LongVector(long _longVector);

// 向向量添加元素
void LongVector_Add(long _longVector, long value);

// 在指定索引插入元素（索引超出范围则忽略）
void LongVector_Insert(long _longVector, int index, long value);

// 删除指定索引的元素（索引无效则忽略）
void LongVector_RemoveAt(long _longVector, int index);

// 删除第一个匹配的元素（返回是否删除成功）
bool LongVector_Remove(long _longVector, long value);

// 检查向量是否包含指定元素
bool LongVector_Contains(long _longVector, long value);

// 获取向量中元素的数量
int LongVector_Size(long _longVector);

// 获取指定索引的元素（索引无效返回 0，可根据需求调整默认值）
long LongVector_Get(long _longVector, int index);

// 修改指定索引的元素（索引无效则忽略）
void LongVector_Set(long _longVector, int index, long value);

// 清空向量中的所有元素
void LongVector_Clear(long _longVector);

// 获取向量中指定元素的第一个索引（不存在返回 - 1）
int LongVector_IndexOf(long _longVector, long value);


// 创建一个新的 double 向量并返回其指针（作为 long 类型）
long DoubleVector();

// 释放 double 向量的内存（返回 0 表示成功，非 0 表示失败）
long Delete_DoubleVector(long _doubleVector);

// 向向量添加元素
void DoubleVector_Add(long _doubleVector, double value);

// 在指定索引插入元素（索引超出范围则忽略）
void DoubleVector_Insert(long _doubleVector, int index, double value);

// 删除指定索引的元素（索引无效则忽略）
void DoubleVector_RemoveAt(long _doubleVector, int index);

// 删除第一个匹配的元素（返回是否删除成功）
bool DoubleVector_Remove(long _doubleVector, double value);

// 检查向量是否包含指定元素（简化版，未考虑精度）
bool DoubleVector_Contains(long _doubleVector, double value);

// 获取向量中元素的数量
int DoubleVector_Size(long _doubleVector);

// 获取指定索引的元素（索引无效返回 0.0）
double DoubleVector_Get(long _doubleVector, int index);

// 修改指定索引的元素（索引无效则忽略）
void DoubleVector_Set(long _doubleVector, int index, double value);

// 清空向量中的所有元素
void DoubleVector_Clear(long _doubleVector);

// 获取向量中指定元素的第一个索引（不存在返回 - 1，未考虑精度）
int DoubleVector_IndexOf(long _doubleVector, double value);


/**
 * 创建新的空Mat
 * @return
 */
long NewMat();


/**
 * 获取图片的宽度
 * @param imgAddress
 * @return
 */
int GetWidth(const long imgAddress);

/**
 * 获取图片的高度
 * @param imgAddress
 * @return
 */
int GetHeight(const long imgAddress);

/**
 * 图片是否为空
 * @param imgAddress
 * @return
 */
bool Empty(const long imgAddress);

/**
 * 回收图片
 * @param img
 * @return
 */
bool Recycle(const long imgAddress);


/**
 * 释放byte[]
 * @param data
 * @return
 */
bool DeleteCharArr(char *data);


/**
 * byte[]转Mat图片
 * @param data byte[]
 * @param height 图片的高
 * @param width 图片宽
 * @param channels 图片通道类型 1,3,4
 * @return
 */
void BytesToMat(char *data, int height, int width, int channels, const long resultAddress);

/**
 * 将 cv::Mat 转换为 char* 原始像素数据
 * @param img 输入的 Mat 对象
 * @param outSize 输出参数，用于返回数据的字节大小
 * @param channels 目标通道类型（1: 灰度图, 3: BGR, 4: BGRA）
 * @return 指向原始数据的 char* 指针（需要手动释放内存）
 */
const char *MatToBytes(const long imgAddress, int *outSize, int channels);


/**
 * 读取文件内容
 * @param path  文件路径
 * @return  返回json字符串（返回值需用 DeleteCharArr 释放)
 */
char *ReadFile(const char *path);

/**
 * 设置字库
 * @param _textPath  字库路径
 * @param _fontName 字库名称
 * @return
 */
bool SetDictByPath(char *_textPath, char *_fontName);

/**
 * 设置字库
 * @param fontContent  字库内容
 * @param _fontName 字库名称
 * @return
 */
bool SetDict(const char *fontContent, const char *_fontName);

/**
 * 切换字库
 * @param _fontName 字库名称
 * @return
 */
bool UseDict(const char *_fontName);
/**
 * 移除字库
 * @param _fontName 字库名称
 * @return
 */
bool RemoveDict(const char *_fontName);


/**
 * 普通找字
 * @param addrRgb 图片
 * @param _text 目标文字
 * @param threshold  相似度
 * @param count 寻找数量
 * @param _fontName 字库名称 写 "" 则使用默认字库
 * @return 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindStr(const long addrRgb, char *_text, int sx, int sy, int ex, int ey, double threshold,
        int count, char *_fontName);


/**
 * 找字扩展
 * @param addrRgb 图片
 * @param _text 目标文字
 * @param threshold  相似度
 * @param count 寻找数量
 * @param _fontName 字库名称  写 "" 则使用默认字库
 * @return 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindStrEx(const long addrRgb, char *_text, int sx, int sy, int ex, int ey, double threshold,
          int count, char *_fontName);


/**
 * 允许用字库里的字任意形式组合找字
 * @param addrRgb 图片
 * @param _text 字符串 如果有多个字符串 允许用 丨隔开 如象棋丨野马丨大炮兵
 * @param sx    起点x 默认0
 * @param sy    起点y 默认0
 * @param ex    终点x 默认0
 * @param ey    终点y 默认0
 * @param threshold 阈值 0-1.0
 * @param count  	寻找个数
 * @param char_interval 字符间隔, 以当前查找字符中最大字符的宽度为单位, 默认是 0.5, 也就是0.5个字符宽度
 * @param global 是否为精准查找,默认值false 为模糊查找。 例如找 “第章” 模糊查找可以找到“第一章”“第十章”，反之不能
 * @param _fontName 指定字库名称 例如 myFontName，写 "" 则使用默认字库
 * @return 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindStrComb(const long addrRgb, char *_text, int sx, int sy, int ex, int ey, double threshold,
            int count, double charInterval, bool global, char *_fontName);


/**
 * 字库识字
 * @param addrRgb  图片
 * @param _colorInfo 颜色
 * @param sx    起点x 默认0
 * @param sy    起点y 默认0
 * @param ex    终点x 默认0
 * @param ey    终点y 默认0
 * @param threshold 阈值
 * @param colorFilter 是否用设定的colorInfo对字库进行筛选,默认 false; 如果是true,有助于提供效率(前提是你的字库的颜色于设定colorInfo一致)
 * @param _fontName 指定字库名称 例如 myFontName，写 "" 则使用默认字库
 * @return 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
GetWords(const long addrRgb, char *_colorInfo, int sx, int sy, int ex, int ey, double threshold,
         bool colorFilter, char *_fontName);

/**
 * 全屏字库识字
 * @param addrRgb  图片
 * @param _colorInfo 颜色
 * @param sx    起点x 默认0
 * @param sy    起点y 默认0
 * @param ex    终点x 默认0
 * @param ey    终点y 默认0
 * @param threshold 阈值
 * @param colorFilter 是否用设定的colorInfo对字库进行筛选,默认 false; 如果是true,有助于提供效率(前提是你的字库的颜色于设定colorInfo一致)
 * @param charInterval 字符间距
 * @param _fontName 指定字库名称 例如 myFontName，写 "" 则使用默认字库
 * @return 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
OcrText(const long addrRgb, char *_colorInfo, int sx, int sy, int ex, int ey, double threshold,
        bool colorFilter, double charInterval,char *_fontName);
/**
 * 单点找色 和EC 用法一样
 * @param img   Mat图片
 * @param color   {string} 要找的颜色如 "0xCDD7E9-0x101010" "#00FF00" "0xCDD7E9-0x101010|0xCDD7E9-0x101010"
 * @param threshold {number} 找色时颜色相似度取值为 0.0 ~ 1.0  默认 0.9
 * @param sx  起点x 默认0;
 * @param sy 起点y 默认0;
 * @param ex  终点x 默认0;
 * @param ey  终点y 默认0;
 * @param count  寻找的个数 默认1
 * @param direction 寻找的方向 1~8 默认1 <br/>
 1: → 从左上横着找到右下 (默认方式)  <br/>
 2: ← 从右上横着找到左下  <br/>
 3: ↓ 从右上竖直找到左下  <br/>
 4: ↑ 从右下竖直找到左上  <br/>
 5: ← 从右下横着找到左上  <br/>
 6: → 从左下横着找到右上  <br/>
 7: ↑ 从左下竖直找到右上  <br/>
 8: ↓ 从左上竖直找到右下  <br/>
 * @returns  返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindColor(const long addrRgb, char *_color, double threshold, int sx, int sy, int ex, int ey,
          int count, int direction);


/**
 *  单点找非色
 * @param img  {Mat} 图片
 * @param colorStr   {string} 不想找的一个或多个颜色 例如 "0xCDD7E9-0x101010" "#00FF00"   "0xCDD7E9-0x101010 & 0x9999E9-0x101010" 主要 & 符合表示 与 ,
 * @param threshold   {number} 找色时颜色相似度取值为 0.0 ~ 1.0  只对不带偏色的颜色值有效(偏色也是一种相似度) 默认0.9  以255为基准 即 255*(1-0.9)=25;
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count {number?} 寻找的个数 默认1
 * @param direction {number?} 寻找的方向 1~8 默认1 <br/>
 1: → 从左上横着找到右下 (默认方式)  <br/>
 2: ← 从右上横着找到左下  <br/>
 3: ↓ 从右上竖直找到左下  <br/>
 4: ↑ 从右下竖直找到左上  <br/>
 5: ← 从右下横着找到左上  <br/>
 6: → 从左下横着找到右上  <br/>
 7: ↑ 从左下竖直找到右上  <br/>
 8: ↓ 从左上竖直找到右下  <br/>
 * @returns  返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindNotColor(const long addrRgb, char *_color, double threshold, int sx, int sy, int ex, int ey,
             int count, int direction);


/**
 * 多点找色 与EC一样的用法
 * @param img {Mat} Mat图片
 * @param firstColor {string} 第一个点的颜色 例如 "0xCDD7E9-0x101010" "#00FF00"
 * @param colorMulti {string} 字符串类似这样 "6|1|0x969696-0x000010,1|12|0x969696,-4|0|0x969696"
 * @param threshold {number?}  找色时颜色相似度取值为 0.0 ~ 1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count {number?} 寻找的个数 默认1
 * @param direction {number?} 寻找的方向 1~8 默认1 <br/>
 1: → 从左上横着找到右下 (默认方式)  <br/>
 2: ← 从右上横着找到左下  <br/>
 3: ↓ 从右上竖直找到左下  <br/>
 4: ↑ 从右下竖直找到左上  <br/>
 5: ← 从右下横着找到左上  <br/>
 6: → 从左下横着找到右上  <br/>
 7: ↑ 从左下竖直找到右上  <br/>
 8: ↓ 从左上竖直找到右下  <br/>
 * @returns  返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *FindMultiColor(const long addrRgb, const char *_firstColor, const char *_colorMulti,
                           double threshold, int sx, int sy, int ex, int ey, int count,
                           int direction);


/**
 * 多点找色 可以指定点颜色是否相等
 * @param img {Mat} Mat图片
 * @param firstColor {string} 第一个点的颜色 例如 "0xCDD7E9-0x101010" "#00FF00"  用于定位用 不可指定相等或不相等,
 * @param colorMulti {string} 字符串类似这样 前缀 1代表相等,0代表不相等 没前缀,默认相等 例如 "1|6|1|0x969696-0x000010,0|6|12|0x969696,-4|5|0x969696"
 * @param threshold {number?}  找色时颜色相似度取值为 0.0 ~ 1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count {number?} 寻找的个数 默认1
 * @param direction {number?} 寻找的方向 1~8 默认1 <br/>
 1: → 从左上横着找到右下 (默认方式)  <br/>
 2: ← 从右上横着找到左下  <br/>
 3: ↓ 从右上竖直找到左下  <br/>
 4: ↑ 从右下竖直找到左上  <br/>
 5: ← 从右下横着找到左上  <br/>
 6: → 从左下横着找到右上  <br/>
 7: ↑ 从左下竖直找到右上  <br/>
 8: ↓ 从左上竖直找到右下  <br/>
 * @returns  返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *FindMultiColorEx(const long addrRgb, const char *_firstColor, const char *_colorMulti,
                             double threshold, int sx, int sy, int ex, int ey, int count,
                             int direction);


/**
 * 与EC的单点比色类似 但有些许不同 EC的points里的点都是绝对位置,而这个方法,只有第一个点是绝对位置,
 * 其他点都是相对于第一个点的相对位置
 *  单点或多点比色
 * @param img  {Mat} 图片  <br/>
 * @param points {string} 颜色字符串 例如"512|833|0x969696-0x000010,1|12|0x969696,-4|0|0x969696" 第一个点为绝对位置,其他点是相对于点一个点的相对位置  <br/>
 * @param threshold  {number?} 匹配度 ,这个值只对不带偏色的点有效果  <br/>
 * @returns {boolean} true 表示纯在 false 表示不纯在  <br/>
 */
bool CmpColor(const long addrRgb, const char *_points, double threshold);

/** 单点比色扩展 可以指定点颜色是否相等
 * 与EC的单点比色类似 但有些许不同 EC的points里的点都是绝对位置,而这个方法,只有第一个点是绝对位置,
 * 其他点都是相对于第一个点的相对位置
 *  单点或多点比色
 * @param img  {Mat} 图片
 * @param points {string} 颜色字符串 前缀0代表不相等 1代表相等 没前缀默认相等  例如"0|512|833|0x969696-0x000010,1|12|0x969696,1|-4|0|0x969696" 第一个点为绝对位置,其他点是相对于点一个点的相对位置
 * @param threshold  {number?} 匹配度 ,这个值只对不带偏色的点有效果
 * @returns {boolean} true 表示纯在 false 表示不纯在
 */
bool CmpColorEx(const long addrRgb, const char *_points, double threshold);


/**
 * 多点数组比色  与Ec的区别 同上述 cmpColor;
 * @param img   {Mat} 图片
 * @param pointsArr {string[]} 颜色字符串数组 具体参考EC 的写法 注意区别
 * @param threshold {number?} 匹配度 ,这个值只对不带偏色的点有效果
 * @returns {number} 如果找到就返回当前points的索引值，如果返回-1，说明都没有找到}
 */
int CmpMultiColor(const long addrRgb, long _pointsArr, double threshold);

/**
 *  快速找图
 * @param img {Mat} Mat或者Mat大图
 * @param temp {Mat} Mat目标小图
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true,默认为false
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgFast(const long addrRgb, const long addrTemp, double threshold, int sx, int sy, int ex,
            int ey, int count, bool strictMode);

/**
 *  快速找图 弱化版 (效果和ec aj找图一样), 相比于 findImgFast 没有 颜色校验 明暗校验 哈希校验, 对找半透明且背景变化的图有利,失去了严格的校验也存在找错的可能
 * @param img {Mat} Mat大图
 * @param temp {Mat} Mat目标小图
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgFastWeak(const long addrRgb, const long addrTemp, double threshold, int sx, int sy, int ex,
                int ey, int count);
/**
 *  快速找图
 * @param img {Mat} Mat或者Mat大图
 * @param temp {Mat[]} Mat小图数组
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true,默认为false
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgFastArr(const long addrRgb, const long addrTemps, double threshold, int sx, int sy, int ex,
               int ey, int count, bool strictMode);

/**
 *  快速找图 弱化版 (效果和ec aj找图一样), 相比于 findImgFast 没有 颜色校验 明暗校验 哈希校验, 对找半透明且背景变化的图有利,失去了严格的校验也存在找错的可能
 * @param img {Mat} Mat大图
 * @param temp {Mat[]} Mat小图数组
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgFastWeakArr(const long addrRgb, const long addrTemps, double threshold, int sx, int sy,
                   int ex, int ey, int count);


/**
 *  基于彩色的带方向的找图
 * @param img {Mat} Mat大图
 * @param temp {Mat} Mat目标小图
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true(极为耗时),默认为false
 * @param direction {number?}  取值 0~8,带方向耗时,默认为0 不带方向
 0: 不带方向 <br/>
 1: → 从左上横着找到右下   <br/>
 2: ← 从右上横着找到左下  <br/>
 3: ↓ 从右上竖直找到左下  <br/>
 4: ↑ 从右下竖直找到左上  <br/>
 5: ← 从右下横着找到左上  <br/>
 6: → 从左下横着找到右上  <br/>
 7: ↑ 从左下竖直找到右上  <br/>
 8: ↓ 从左上竖直找到右下  <br/>
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgFastEx(const long addrRgb, const long addrTemp, double threshold, int sx, int sy, int ex,
              int ey, int count, bool strictMode, int direction);

/**
 *  基于彩色的带方向的找图
 * @param img {Mat} Mat大图
 * @param temp {Mat[]} Mat[]目标小图
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true(极为耗时),默认为false
 * @param direction {number?}  取值 0~8,带方向耗时,默认为0 不带方向
 0: 不带方向 <br/>
 1: → 从左上横着找到右下   <br/>
 2: ← 从右上横着找到左下  <br/>
 3: ↓ 从右上竖直找到左下  <br/>
 4: ↑ 从右下竖直找到左上  <br/>
 5: ← 从右下横着找到左上  <br/>
 6: → 从左下横着找到右上  <br/>
 7: ↑ 从左下竖直找到右上  <br/>
 8: ↓ 从左上竖直找到右下  <br/>
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgFastExArr(const long addrRgb, const long addrTemps, double threshold, int sx, int sy, int ex,
                 int ey, int count, bool strictMode, int direction);
/**
 *  基于灰度的带方向的找图
 * @param mat {Mat} mat大图
 * @param temp {Mat} Mat目标小图
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true(极为耗时),默认为false
 * @param direction {number?}  取值 0~8,带方向耗时,默认为0 不带方向
 0: 不带方向 <br/>
 1: → 从左上横着找到右下   <br/>
 2: ← 从右上横着找到左下  <br/>
 3: ↓ 从右上竖直找到左下  <br/>
 4: ↑ 从右下竖直找到左上  <br/>
 5: ← 从右下横着找到左上  <br/>
 6: → 从左下横着找到右上  <br/>
 7: ↑ 从左下竖直找到右上  <br/>
 8: ↓ 从左上竖直找到右下  <br/>
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgFastWeakEx(const long addrRgb, const long addrTemp, double threshold, int sx, int sy, int ex,
                  int ey, int count, bool strictMode, int direction);
/**
 *  基于灰度的带方向的找图
 * @param mat {Mat} mat大图
 * @param temp {Mat[]} Mat[]目标小图
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true(极为耗时),默认为false
 * @param direction {number?}  取值 0~8,带方向耗时,默认为0 不带方向
 0: 不带方向 <br/>
 1: → 从左上横着找到右下   <br/>
 2: ← 从右上横着找到左下  <br/>
 3: ↓ 从右上竖直找到左下  <br/>
 4: ↑ 从右下竖直找到左上  <br/>
 5: ← 从右下横着找到左上  <br/>
 6: → 从左下横着找到右上  <br/>
 7: ↑ 从左下竖直找到右上  <br/>
 8: ↓ 从左上竖直找到右下  <br/>
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgFastWeakExArr(const long addrRgb, const long addrTemps, double threshold, int sx, int sy,
                     int ex, int ey, int count, bool strictMode, int direction);

/**模板匹配找图
 *  与EC找图用法类似  注意返回值 这里做成了和EC一样
 * @param mat {Mat} Mat大图
 * @param temp {Mat} Mat目标小图
 * @param resize  {number?} 大图缩放倍数 适当缩放有利于速度提升 例如 1表示不缩放 0.5表示缩小一倍 默认0.5
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count   {number?} 寻找个数 默认 1
 * @param mask    {Mat?} 掩膜 对应的是小图 与小图一样大小 默认null
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImg(const long addrRgb, const long addrTemp, double resize, double threshold, int sx, int sy,
        int ex, int ey, int count, const long addrMask);


/**
 * 只做图片的轮廓匹配,无视颜色  注:这个不是全分辨率
 * @param img {Mat} Mat大图
 * @param temp   {Mat} Mat目标小图
 * @param resize  {number?} 大图缩放倍数 适当缩放有利于速度提升 例如 1表示不缩放 0.5表示缩小一倍 默认0.5
 * @param threshold {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count {number?} 寻找个数 默认 1
 * @param weakThreshold {number?}  检测轮廓(canny)时的弱阈值 大于弱阈值且依附在边缘的点会被保留  默认值 50
 * @param strongThreshold {number?} 检测轮廓(canny)时的强阈值 即相邻的像素点差值大于强阈值则被认为是边缘 默认值 100
 *  @return 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgByContour(const long addrRgb, const long addrTemp, double resize, double threshold, int sx,
                 int sy, int ex, int ey, int count, int weakThreshold, int strongThreshold);


/** 半透明找图 类似 按键精灵的半透明找图
 *  模板 背景最好为纯色,如果是杂色,请涂成纯色,此方法认为与模板四个脚颜色相同的点为背景,将忽略
 *
 * @param mat {Mat} mat大图
 * @param temp {Mat} mat目标小图
 * @param resize  {number?} 大图缩放倍数 适当缩放有利于速度提升 例如 1表示不缩放 0.5表示缩小一倍 默认0.5
 * @param threshold {number?} 匹配度 0~1 默认0.95
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count   {number?} 寻找个数 默认 1
 * @param colorOffset {number?} 背景颜色偏差 默认 20 个点
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindTranslucentImg(const long addrRgb, const long addrTemp, double resize, double threshold, int sx,
                   int sy, int ex, int ey, int count, int colorOffset);


/**
 * 创建图片
 * @param width 宽
 * @param height 高
 * @param _color 颜色
 * @param resultAddress 结果
 */
void CreateMat(int width, int height, const char *_color, const long resultAddress);

/**  surf算法全分辨率找图
 * 对图片进行缩放有利于提高速度，缩放过了就会因太模糊而找不到图
 * 需自行测试最佳比例，使用中等分辨率做模板最佳(其他分辨率也可以,中等分辨率对高低兼容性强而已;),
 * @param img {Mat}  Mat大图
 * @param template {Mat}  Mat目标小图
 * @param imgResize {number?} 大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempResize {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param threshold {number?} 匹配度  默认是0.6
 *                             注：这个匹配度 是先找到位置再算匹配度 而ec模板匹配是先算匹配度再确认位置
 *                             所以这个匹配度只是用于最终确认用的,写的更低也是ok的比如 0.5,其他全分辨率同理。
 * @param upright {boolean?} 只计算直立方向？true 是， false 计算全部方向（即旋转不变性，耗时） 默认true
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgBySurf(const long addrRgb, const long addrTemp, double imgResize, double tempResize, int sx,
              int sy, int ex, int ey, double threshold, bool upright);


/**
 *原始surf找图 具有尺度不变性 旋转不变性 光照不变性
 * @param img {Mat} Mat大图
 * @param temp  {Mat} Mat目标小图
 * @param srcMultiple {number?} 大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempMultiple {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param upright  {boolean?} 只计算直立方向？true 是， false 计算全部方向（即旋转不变性，耗时） 默认true
 * @param distanceRatio {number?} 距离比值 最近距离和次最近距离的比值 可以理解为准确度 这个方法没有匹配度而是使用距离比值,值范围在 0.4~0.6之间最佳,对准确度要求高的写0.4,一般写0.5,默认值0.6
 * @param customFeatures  {number?} 特征匹配对 阈值, 用于定义 至少多少对特征点匹配成功,才认为是目标. 特征点个数与模板大小成正比,默认 6 对
 * @param minHessian  {number?}    minHessian越大特征点越稳定 同时也越少,默认值100
 * @param nOctaves    {number?}   图像金字塔组数,默认值 4
 * @param nOctaveLayers {number?}   金字塔每组层数,默认值 3
 * @param extended   {boolean?}    表示特征向量的维度, false 表示64维 速度快,true 表示128维 准确性高,默认 false;
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgBySurfEx(const long addrRgb, const long addrTemp, double srcMultiple, double tempMultiple,
                int sx, int sy, int ex, int ey, bool upright, double distanceRatio,
                int customFeatures, int minHessian, int nOctaves, int nOctaveLayers, bool extended);


/** sift算法全分辨率找图
 *对图片进行缩放有利于提高速度，缩放过了就会因太模糊而找不到图
 * 需自行测试最佳比例，使用中等分辨率做模板最佳(其他分辨率也可以,中等分辨率对高低兼容性强而已;),
 * @param {Mat} img  Mat大图
 * @param {Mat} template Mat目标小图
 * @param imgResize {number?} 大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempResize {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param threshold  {number?} 匹配度 0~1 默认0.6
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgBySift(const long addrRgb, const long addrTemp, double imgResize, double tempResize, int sx,
              int sy, int ex, int ey, double threshold);


/**
 * 原始sift找图 具有尺度不变性 旋转不变性 光照不变性  无视背景
 * @param img {Mat} Mat大图
 * @param temp {Mat} Mat目标小图
 * @param srcMultiple {number?}大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempMultiple {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param distanceRatio {number?} 距离比值 最近距离和次最近距离的比值 可以理解为准确度 这个方法没有匹配度而是使用距离比值,值范围在 0.4~0.6之间最佳,对准确度要求高的写0.4,一般写0.5,默认值0.6
 * @param customFeatures  {number?} 特征匹配对 阈值, 用于定义 至少多少对特征点匹配成功,才认为是目标. 特征点个数与模板大小成正比,默认 6 对
 * @param nFeatures  {number?}  表示要返回的最优秀特征点数量  默认 0;
 * @param nOctaveLayers  {number?} 每组图像的层数(这个方法图像金字塔的组数是自动计算的) 默认 3;
 * @param contrastThreshold {number?} 对比度阈值 阈值越大，检测器产生的特征越少。 默认 0.04;
 * @param edgeThreshold    {number?}  边缘阈值 ，阈值越大，被过滤掉的越少，特征点越多  默认10;
 * @param sigma    {number?}   金字塔第0层图像高斯滤波系数，也就是σ。默认1.6;
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgBySiftEx(const long addrRgb, const long addrTemp, double srcMultiple, double tempMultiple,
                int sx, int sy, int ex, int ey, double distanceRatio, int customFeatures,
                int nFeatures, int nOctaveLayers, double contrastThreshold, double edgeThreshold,
                double sigma);

/**ORB算法全分辨率找图
 *  //此方法速度快 但比较难以控制,看实验
 * @param img {Mat} Mat大图
 * @param template {Mat} Mat目标小图  这个方法对长宽要求不能小于40个像素
 * @param imgResize {number?} 大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempResize {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param threshold  {number?} 匹配度 0~1 默认0.7
 * @param nfeatures  {number?} 选择特征点数  默认值 10000个 如果找不到目标可以试着把这个值设的更高 比如100000
 * @param scaleFactor {number?} 图像金字塔每层缩放倍数 默认值 1.2 必须大于1
 * @param nlevels  {number?} 图像金字塔组数 默认值 8  可以简单的理解为 nlevels*scaleFactor 越大 适应的分辨率越宽
 * @returns 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
FindImgByORB(const long addrRgb, const long addrTemp, double imgResize, double tempResize, int sx,
             int sy, int ex, int ey, double threshold, int nfeatures, double scaleFactor,
             int nlevels);


/**
 * 全分辨率形状匹配
 * @param  img  {Mat} Mat图片
 * @param _colors 颜色 由图色工具生成的属性对象
 * @param _imageMode  模式 由图色工具生成的属性对象
 * @param _info  矩阵信息 由图色工具生成的属性对象
 * @param _hash  哈希值 由图色工具生成的属性对象
 * @param threshold  {number?} 相识度 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 寻找数量 默认1个
 * @param minArea {number?} 最小面积,指轮廓围成的面积,默认200
 * @param maxArea {number?} 最大面积,指轮廓围成的面积,默认-1 表示不限制
 * @return 返回json字符串（返回值需用 DeleteCharArr 释放)
 */
const char *
MatchShape(const long addrRgb, const char *_colors, const char *_imageMode, const long _info,
           const char *_hash, double threshold, int sx, int sy, int ex, int ey, int count,
           double minArea, double maxArea);


//见js版本的findImgByColor方法
const char *GetColorMap(const char *_path);

const char *
FindImgColor(const long addrRgb, const char *_tempPath, double threshold, int sx, int sy, int ex,
             int ey, int count, int direction);

const char *
FindImgColor2(const long addrRgb, double srcMultiple, const long addrTemp, double tempMultiple,
              double distanceRatio, int customFeatures, const char *_tempPath, const char *_color,
              const char *_colorMulti, double threshold, int sx, int sy, int ex, int ey, int count,
              int direction);

//金字塔模版匹配
const char *
FindImgByPyramid(const long addrRgb, const long addrTemp, int sx, int sy, int ex, int ey,
                 int matchMethod, double weakThreshold, double strictThreshold, int count,
                 int scaleTimes, double scalePro);
/**
 * 加载yaml 文件
 * @param names {string?} yaml文件名称允许使用通配符“*” 多个用"|"隔开  例如 "yamlOne|yaml*|weixin",如果为 空 或不写 则加载path下所有 yaml文件;
 * @param path {string?} 路径  可以是外部的 也可以是内部脚本目录下  例如 "/storage/emulated/0/Pictures" , "drawable"; 默认为 ""(脚本同目录);
 * @return bool
 */
bool LoadYaml(const long _names, const long _yamlArr);

//返回 “010101” 字符串 0为false 为true
char *YamlExist(const long _names);

//释放yaml
bool ReleaseYaml(const long _names);

/**
 *  梯度特征匹配   可全分,可测角度,无视色差,可返回多个不同目标
 * @param img {Mat} Mat图片;
 * @param names {string} yaml文件名称 多个用"|"隔开  例如 "yamlOne|yamlTow|weixin",如果不写 则查找res下所有yaml目标(不包含子目录);
 * @param sx  {number?} 起点x 默认0;
 * @param sy  {number?} 起点y 默认0;
 * @param ex  {number?} 终点x 默认0;
 * @param ey  {number?} 终点y 默认0;
 * @param threshold {number?} 阈值 0~1,默认 0.9;
 * @param count {number?} 寻找个数,默认 1;
 * @param step   {number[]?}  金字塔梯度 默认为[4,8],可选值 [2,4] ,[4,8];
 * @param NMSThreshold {number?} 非极大值抑制阈值 默认0.3; 即两个目标重叠时 面积交集/并集>阈值 将被舍去
 * @param mask {Mat?} 大图的掩膜 默认null;
 * @return 返回字符串（返回值需用 DeleteCharArr 释放)
 */
char *
GradientShapeMatch(const long addrRgb, const long _nameArr, const long _dataArr, double threshold,
                   int sx, int sy, int ex, int ey, int count, const long _step, double NMSThreshold,
                   const long maskAddr);

/**
 * 返回指定点的颜色字符串大写
 * @param img {Mat} 图片
 * @param x {number} 坐标 x
 * @param y {number} 坐标 y
 * @returns 返回字符串（返回值需用 DeleteCharArr 释放)
 */
const char *GetPixelColor(const long addrRgb, int x, int y);

/**
 * 获取指定点的 Red 通道颜色
 * @param img {Mat} 图片
 * @param x {number} 坐标 x
 * @param y {number} 坐标 y
 * @returns {number} 0~255 范围的颜色值
 */
int GetRed(const long addrRgb, int x, int y);
/**
 * 获取指定点的 Green 通道颜色
 * @param img {Mat} 图片
 * @param x {number} 坐标 x
 * @param y {number} 坐标 y
 * @returns {number} 0~255 范围的颜色值
 */
int GetGreen(const long addrRgb, int x, int y);
/**
 * 获取指定点的 Blue 通道颜色
 * @param img {Mat} 图片
 * @param x {number} 坐标 x
 * @param y {number} 坐标 y
 * @returns {number} 0~255 范围的颜色值
 */
int GetBlue(const long addrRgb, int x, int y);


/** 图片伸缩
 * @param mat  {Mat} mat原图
 * @param newWidth {number} 新宽度
 * @param newHeight {number} 新高度
 * @returns {Mat}
 */
void ScaleMatByWH(const long addrRgb, const long addrResult, int width, int height);
/** 图片伸缩
 * @param mat {Mat} mat原图
 * @param ratioX  {number} X轴伸缩比例 如0.5 1.3 等
 * @param ratioY {number} Y轴伸缩比例
 * @returns {Mat}
 */
void ScaleMatByRatio(const long addrRgb, const long addrResult, double ratioX, double ratioY);


/** 图片旋转
 * 需要自己管理图片的释放
 * @param mat {Mat} mat原图
 * @param alpha  {number} 角度 如 90  旋转90度
 * @returns {Mat|null}
 */
void RotateMat(const long addrRgb, const long addrResult, int alpha);


void CropMat(const long addrRgb, const long addrResult, int sx, int sy, int ex, int ey);

/**
 * 图片叠加
 * @param img1 {Mat} 图片1
 * @param img2 {Mat} 图片2
 * @param x {number?} 起点x 默认0;
 * @param y {number?} 起点y 默认0;
 * @param alpha {number?} 图片2 的透明度,范围 0~1,默认1 不透明
 * @param mask {Mat?} 与图片2一样大小的二值化图片,默认null
 * @returns {null|Mat}
 */
void OverlyingMat(const long addrRgb, const long addrTemp, const long addrResult, int sx, int sy,
                  double alpha, const long addrMask);


/**
 * 画矩形
 * @param img {Mat} 图片
 * @param sx  {number} 起点x
 * @param sy  {number} 起点y
 * @param ex  {number} 终点x
 * @param ey  {number} 终点y
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2, -1表示填充
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 4 不抗锯齿
 */
void DrawRect(const long addrRgb, int left, int top, int right, int bottom, const char *_color,
              int thickness, int lineType);


/**
 *   图片上画圆
 * @param img {Mat} 图片
 * @param x   {number}  圆心x
 * @param y    {number}  圆心y
 * @param r    {number} 半径r
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2, -1表示填充
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 16 抗锯齿
 **/
void DrawCircle(const long addrRgb, int x, int y, int r, const char *_color, int thickness,
                int lineType);


/**
 *
 * @param img {Mat} 图片
 * @param sx  {number} 起点x
 * @param sy  {number} 起点y
 * @param ex  {number} 终点x
 * @param ey  {number} 终点y
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param lineWidth {number?} 画笔粗细  默认 2
 */
void
DrawLines(const long addrRgb, int sx, int sy, int ex, int ey, const char *_color, int thickness,
          int lineType);

/**
 * 统计非零的像素点个数
 * @param img  {Mat} 图片
 * @returns {number|null}  非零像素点个数
 */
int CountNonZero(const long addrGray);


/**
 *  图片镜像
 * @param img  {Mat} Mat大图
 * @param mode  镜像模式 1:左右镜像; 0:上下左右镜像;-1上下镜像
 * @returns {Mat} 镜像图片
 */
void Flip(const long addrRgb, const long addrResult, int mode);


/**
 * 感知哈希算法
 * @param mat {Mat} Mat图片
 * @param dctSize  {number?}  离散余弦变换大小,可以理解为缩放后的图片大小 用于DCT计算 默认32 即32*32大小 可以选值 8,16,32 值越小速度越快
 * @param ROISize  {number?}  对DCT的结果取值大小,默认8,即 8*8大小
 * @returns {string} 哈希值 或者 ""
 */
const char *PHashCode(const long addrGray, int dctSize, int ROISize);


/**
 * 汉明距离算法
 * @param str1 {string} 字符串1
 * @param str2 {string} 字符串2
 * @returns {number}  数字越大代表差别越大
 * @constructor
 */
int HanMingDistance(const char *str1, const char *str2);


/**
 * 马赛克
 * @param srcMat {Mat} 图片
 * @param stride {number?} 马赛克大小 默认10;
 * @param sx  {number?} 起点x 默认0;
 * @param sy  {number?} 起点y 默认0;
 * @param ex  {number?} 终点x 默认0;
 * @param ey  {number?} 终点y 默认0;
 * @returns {Mat}
 */
void Masaike(const long addrRgb, const long addrResult, int stride, int sx, int sy, int ex, int ey);


//获取版本 返回字符串（返回值需用 DeleteCharArr 释放)
const char *Version();


/**
 * 图片转灰度
 * @param mat  {Mat} mat原图
 * @returns  {Mat|null}
 */
void MatToGray(const long addrRgb, const long addrResult);


/**
 * 图片二值化 参考EC的二值化
 * @param mat {Mat} Mat原图
 * @param threshold_value {number} 二值化阈值，0 ~ 255
 * @param maxValue {number?} 大于阈值的点变成的数值 默认 255 白色;
 * @param type  一般写0即可 默认0<br/>
 0 灰度值大于阈值为最大值，其他值为0<br/>
 1 灰度值大于阈值为0，其他值为最大值<br/>
 2 灰度值大于阈值的为阈值，其他值不变<br/>
 3 灰度值大于阈值的不变，其他值为0<br/>
 4 灰度值大于阈值的为零，其他值不变<br/>
 8 大津法自动寻求全局阈值<br/>
 16 三角形法自动寻求全局阈值<br/>
 * @returns {Mat}
 */
void BinaryzationMat(const long addrRgb, const long addrResult, int thresholdValue, int maxValue,int type);


/**
 * 双阈值二值化
 * @param mat {Mat} Mat原图
 * @param lowerThreshold  {number} 低阈值，0 ~ 255
 * @param upperThreshold  {number} 高阈值，0 ~ 255
 * @param maxValue  {number?} 大于阈值的点变成的数值 默认 255 白色;
 * @returns {Mat}
 */
void DoubleBinaryzation(const long addrRgb, const long addrResult, int lowerThreshold,
                        int upperThreshold, int maxValue);


/**
 * 自适应二值化
 * @param img {Mat} 图片
 * @param adaptiveMethod {number?} 阈值计算方式 选则值为 0或1, 0-通过平均的方法取得平均值做阈值,1-通过高斯取得高斯值做阈值,默认0
 * @param blockSize  {number?}  计算阈值的领域大小,此值对二值化的最终效果起主要作用, 默认11;
 * @param maxValue   {number?}  二值化最大值 默认255;
 * @param type  {number?}一般写0即可 默认0<br/>
 0 灰度值大于阈值为最大值，其他值为<br/>
 1 灰度值大于阈值为0，其他值为最大值<br/>
 * @param C {number?} 偏移值调整量 默认0, 像素值减去C再与阈值比较
 * @returns {Mat|null} 二值化图片
 */
void AdaptiveThreshold(const long addrRgb, const long addrResult,  int adaptiveMethod, int blockSize,int maxValue, int type, int c);


/**
 * 边缘检测
 * @param img  {Mat} 图片
 * @param threshold1  {number?} 弱阈值 默认50
 * @param threshold2 {number?}  强阈值 默认 100
 * @param apertureSize {number?} Sobel大小 默认3
 * @param L2gradient  {Boolean?} 是否更加精确的计算  默认 false
 * @return {Mat} 边缘图片
 */
void
Canny(const long addrRgb, const long addrResult, int threshold1, int threshold2, int apertureSize,
      bool L2gradient);


/**
 *  直线检测
 * @param img {Mat} Mat图片
 * @param src_multiple {number?} 缩放比例  1不缩放 0.5 缩小一倍 适当缩放有例如加快速度 默认 1.0
 * @param rho    {number?}  步长 即检测的每次跨越的像素点  默认值 1
 * @param theta   {number?} 角度,即每次检查的角度,比如90 则只检测水平和竖直的线 默认值 1
 * @param threshold {number?} 即连续检测到 多少个rho点则认为是一条直线  默认值 10
 * @param minLineLength {number?} 只有大于这个最小长度的直线被返回  默认值 50
 * @param maxLineGap   {number?}  两直线之间缺了多少个像素 也认为它们是同一条直线 默认 5
 * @param weakThreshold {number?}  检测轮廓(canny)时的弱阈值 大于弱阈值且依附在边缘的点会被保留  默认值 50
 * @param strongThreshold {number?} 检测轮廓(canny)时的强阈值 即相邻的像素点差值大于强阈值则被认为是边缘 默认值 100
 * @return {Point[]|null} 返回Point数组 注意两个点为一条直线,或者 null
 */
const char *FindLines(const long addrRgb, double src_multiple, int rho, double theta, int threshold,
                      double minLineLength, double maxLineGap, int cannyWeakThreshold,
                      int cannyStrongThreshold);


/**
 * 圆形检测
 * @param img {Mat} Mat 图片
 * @param minDist {number}  圆心之间最小距离,能解决同心圆问题
 * @param accuracy {number}  找圆的精度 范围为0~1.0,越接近1越像圆
 * @param minRadius {number}  找圆最小半径
 * @param maxRadius {number}  找圆最大半径
 * @param cannyThreshold {number?} 边缘阈值 默认值 100,即转成单通道后的相邻像素点的值相差多少 认为是边界点
 * @param dp  {number?} 累加器图像的分辨率 直接写1.5即可
 * @returns  {PointEx[]|nil}  返回PointEx数组 继承自Point  例如: 圆心坐标:points[0].x ,points[0].y  圆的半径: points[0].r
 */
const char * FindCircles(const long addrRgb,int method,int minDist, double accuracy,int minRadius, int maxRadius,double cannyThreshold,  double dp  );

/**
 * 获取HSV图像或其中一种颜色图像  释放方式 myMat.recycle();
 * @param img {Mat} 图片
 * @param type {number}
 *          -1 表示获取 HSV图像,
 *           0 表示获取H通道图像,
 *           1 表示获取S通道图像,
 *           2 表示获取V通道图像
 * @returns {Mat}  返回Mat图像
 */
void GetHSV(const long addrRgb, const long addrResult, int type);

/**
 * 获取HLS图像或其中一种颜色图像
 * @param img {Mat} 图片
 * @param type {number}
 *          -1 表示获取 HLS图像,
 *           0 表示获取H通道图像,
 *           1 表示获取L通道图像,
 *           2 表示获取S通道图像
 * @returns {Mat}  返回Mat图像
 */
void GetHLS(const long addrRgb, const long addrResult, int type);

/**
 * 获取BGR图像或其中一种颜色图像
 * @param img {Mat} 图片
 * @param type {number}
 *          -1 表示获取 BGR图像,
 *           0 表示获取B通道图像,
 *           1 表示获取G通道图像,
 *           2 表示获取R通道图像
 * @returns {Mat}  返回Mat图像
 */
void GetBGR(const long addrRgb, const long addrResult, int type);


/**
 * 多边形检测
 * @param img  {Mat} Mat二值化图像或者轮廓
 * @param Angles  {number} 目标角数/边数 ,如三角形是3 五边形是5
 * @param compareMode  {number?}  比较方式, -1 表示小于Angles,0表示等于Angles,1表示大于Angles,默认为0
 * @param minArea    {number?}  目标的最小面积 默认为100
 * @param maxArea    {number?}  目标的最大面积 默认为-1 无上限
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param epsilon   {number?} 轮廓近似逼近阈值 默认为 -1 表示目标周长的百分之一为阈值 ： <br/>
 * @returns {number[]|null} 返回的是引索数组,配合其他轮廓函数使用,比如轮廓重心,轮廓外接矩形等
 */
void PolygonDetection(const long addrRgb, const long _resultIntVec, int angles, int compareMode,
                      int minArea, int maxArea, int mode, int method, double epsilon);


/**
 * 获取轮廓的凸包
 * @param img  {Mat} Mat二值化图像或者轮廓
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param thickness  {number?} 绘制的线条宽度 -1表示填充,默认1
 * @param lineType   {number?} 线条类型  8 不抗锯齿 ,16抗锯齿,默认 8
 * @returns {Mat}  凸包图片
 */
void GetConvexHull(const long addrRgb, const long addrResult, int mode, int method,
                   const long _contoursIndex, int thickness, int lineType);


/**
 * 判断轮廓是否是凸包
 * @param img  大图 二值化图片或者轮廓
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param epsilon        {number?} 轮廓近似逼近阈值 默认为 -1 表示目标周长的百分之一为阈值 ： <br/>
 * @return {boolean[]} 返回 boolean 数组  true表示是凸包,false表示不是凸包
 */
const char *IsContourConvex(const long addrRgb, int mode, int method, const long _contoursIndex,
                            double epsilon);


/**
 * 轮廓近似逼近
 * @param img  {Mat} Mat二值化图像或者轮廓
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param thickness  {number?} 绘制的线条宽度 -1表示填充,默认1
 * @param lineType   {number?} 线条类型  8 不抗锯齿 ,16抗锯齿,默认 8
 * @param epsilon        {number?} 轮廓近似逼近阈值 默认为 -1 表示目标周长的百分之一为阈值 ： <br/>
 * @returns {Mat|null}    图片
 */
void ApproxPolyDP(const long addrRgb, const long addrResult, int mode, int method,
                  const long _contoursIndex, int thickness, int lineType, double epsilon);


/**
 * 获取轮廓上的点
 * @param img   {Mat}  图片 一般为二值化图片
 * @param mode  {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要获取的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param epsilon        {number?} 轮廓近似逼近阈值 默认为 -1 表示目标周长的百分之一为阈值 ： <br/>
 * @returns {Point[][]} Point的二维数组 或者 null
 * @constructor
 */
void GetApproxPolyDPPoint(const long addrRgb, const long _pointVector2D, int mode, int method,
                          const long _contoursIndex, double epsilon);


/**
 * 绘制轮廓
 * @param img   {Mat}  图片 一般为二值化图片
 * @param mode  {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param thickness  {number?} 绘制的线条宽度 -1表示填充,默认1
 * @param lineType   {number?} 线条类型  8 不抗锯齿 ,16抗锯齿,默认 8
 * @returns {Mat}
 */
void GetContours(const long addrRgb, const long addrResult, int mode, int method,
                 const long _contoursIndex, int thickness, int lineType);

/**
 *  获取轮廓外接矩形
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {Rect[]|null}   返回Rect[] 或者 null
 */
const char *GetContoursRect(const long addrRgb, int mode, int method, const long _contoursIndex);


/**
 *  轮廓外接矩形扩展
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param dis_x {number?} 外接矩形间的的x轴距离,小于dis_x的矩形将合并,默认5; 若dis_x和dis_y 都为0,则等同于getContoursRect
 * @param dis_y {number?} 外接矩形间的的y轴距离,小于dis_y的矩形将合并,默认5
 * @returns {Rect[]|null}   返回Rect[] 或者 null
 */
const char *
GetContoursRectEx(const long addrRgb, int mode, int method, const long _contoursIndex, int dis_x,
                  int dis_y);


/**
 *  获取轮廓的最小外接矩形
 * @param srcMat  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {Point[][]|null}   返回Point的二维数组 或者 null
 */
void getContoursMinRect(const long addrRgb, const long _pointVector2D, int mode, int method,
                        const long _contoursIndex);


/**
 *  获取轮廓外接圆
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {PointEx[]|null}  返回PointEx数组 内有 x点,y点,r半径  或者 null;
 */
const char *GetContoursCircles(const long addrRgb, int mode, int method, const long _contoursIndex);


/**
 *  获取轮廓长度
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {number[]|null}   返回所有轮廓的长度数组 或者 null
 */
void GetContoursLength(const long addrRgb, const long resultDoubleVec, int mode, int method,const long _contoursIndex);


/**
 *  获取轮廓面积
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {number[]|null}   返回所有轮廓的面积数组 或者 null
 */
void GetContoursArea(const long addrRgb, const long resultDoubleVec, int mode, int method, const long _contoursIndex);

/**
 *  获取轮廓拓扑信息 (即位置信息)
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @returns {number[][]|null} 返回二位数组  <br/>一个数组包含 nextIndex(下一个引索),beforeIndex(上一个引索),childIndex(子引索),parentIndex(父引索)四个引索 例如 [[2,0,5,-1]]  其中-1代表没有
 */
void GetContoursHierarchy(const long addrRgb, const long resultIntVec2D, int mode, int method);


/**
 * 图片的腐蚀操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations       {number?} 迭代次数 即重复此操作的次数 默认1次
 * @return {Mat|null} 腐蚀后图片
 */
void Erode(const long addrRgb, const long addrResult, int structType, int struct_width,
           int struct_height, int anchor_x, int anchor_y, int iterations);

/**
 * 图片的膨胀操作
 * @param img {Mat} 图片
 * @param structType    {number?}     结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width    {number?}   结构元素的宽度 默认 5
 * @param struct_height   {number?}   结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y        {number?}  结构元素的锚点y 默认-1
 * @param iterations     {number?} 迭代次数 即重复此操作的次数 默认1次
 * @return {Mat|null} 膨胀后图片
 */
void Dilate(const long addrRgb, const long addrResult, int structType, int struct_width,
            int struct_height, int anchor_x, int anchor_y, int iterations);

/**
 * 图片的开操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations       {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat|null}  图片
 */
void
Open(const long addrRgb, const long addrResult, int structType, int struct_width, int struct_height,
     int anchor_x, int anchor_y, int iterations);

/**
 * 图片的闭操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations       {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat|null} 图片
 */
void Close(const long addrRgb, const long addrResult, int structType, int struct_width,
           int struct_height, int anchor_x, int anchor_y, int iterations);

/**
 * 图片的梯度
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations      {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat|null}  图片
 */
void Gradient(const long addrRgb, const long addrResult, int structType, int struct_width,
              int struct_height, int anchor_x, int anchor_y, int iterations);

/**
 * 顶帽操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations      {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat|null}  图片
 */
void TopHat(const long addrRgb, const long addrResult, int structType, int struct_width,
            int struct_height, int anchor_x, int anchor_y, int iterations);

/**
 * 黑帽操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations      {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat|null}  图片
 */
void BlackHat(const long addrRgb, const long addrResult, int structType, int struct_width,
              int struct_height, int anchor_x, int anchor_y, int iterations);
/**
 * 击中不击中
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations      {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat|null}  图片
 */
void HitMiss(const long addrRgb, const long addrResult, int structType, int struct_width,
             int struct_height, int anchor_x, int anchor_y, int iterations);


/**
 * 帧差法
 * @param img1 {Mat} 图1
 * @param img2 {Mat} 图2 与图1同大小
 * @returns {Mat|null}
 */
void AbsDiff(const long addrRgb1, const long addrRgb2, const long addrResult);


/**
 * 中值滤波
 * @param img {Mat} 图片
 * @param kSize {number?} 中值统计区域大小,如填5,则在5*5的区域内选中值,默认5
 * @returns {Mat||n}  图片
 */
void MedianBlur(const long addrRgb, const long addrResult, int kSize);


/**
 * 高斯模糊 对图像进行模糊,以达到去噪的过程
 * @param img {Mat} 图片
 * @param kSize   {number?} 卷积核大小 只能选 1,3,5,7。 越大约模糊,默认为 3
 * @param sigmaX  {number?}  图片x轴方向的 模糊程度,越大越模糊, 0 表示根据ksize 自动计算 默认0;
 * @param sigmaY  {number?}  图片y轴方向的 模糊程度,越大越模糊, 0 等同sigmaX  默认0;
 * @param borderType {number?} 边缘模式 选项有0~5,16   默认4
 * @returns {Mat|null} 模糊后的图片
 */
void
GaussianBlur(const long addrRgb, const long addrResult, int ksize, double sigmaX, double sigmaY,
             int borderType);


/**
 * 高斯模糊 对图像进行模糊,以达到去噪的过程
 * @param img {Mat} 图片
 * @param kSize   {number?} 卷积核大小 只能选 1,3,5,7。 越大约模糊,默认为 3
 * @param sigmaX  {number?}  图片x轴方向的 模糊程度,越大越模糊, 0 表示根据ksize 自动计算 默认0;
 * @param sigmaY  {number?}  图片y轴方向的 模糊程度,越大越模糊, 0 等同sigmaX  默认0;
 * @param borderType {number?} 边缘模式 选项有0~5,16   默认4
 * @returns {Mat} 模糊后的图片
 */
void
Blur(const long addrRgb, const long addrResult, int ksize_width, int ksize_height, int anchor_x,
     int anchor_y, int borderType);


/**
 *  双边滤波器
 * @param img {Mat} 图片
 * @param d  {number} 滤波过程中每个像素邻域的直径，如果这个值是非正数，则由参数sigmaSpace计算得到。
 * @param sigmaColor  颜色空间滤波器的标准差值，这个参数越大表明该像素领域内有越多的颜色被混合到一起，产生较大的半相等颜色区域。
 * @param sigmaSpace  空间坐标中滤波器的标准差值，这个参数越大表明越远的像素会相互影响，从而使更大领域中有足够相似的颜色获取相同的颜色。当第三个参数d大于0时，邻域范围由d确定，当第三个参数小于等于0时，邻域范围正比于这个参数的数值。
 * @param borderType  像素外推法选择标志，取值范围在表3-5中给出，默认参数为4，表示不包含边界值倒序填充。
 * @returns {null|Mat}
 */
void
Bilateral(const long addrRgb, const long addrResult, int d, double sigma_color, double sigma_space,
          int border_type);


/**
 *  对图片进行锐化,使图棱角分明 ,可以理解成上述模糊的反向过程
 * @param img {Mat} 图片
 * @param kernelType {number?} 卷积核的类型  默认 1 <Br/>
 *                       1:锐化卷积核,
 *                       2:robert_x算子,
 *                       3:robert_y算子,
 *                       4:Sobel_x算子
 *                       5:Sobel_y算子
 *                       6:拉普拉斯算子 <Br/>
 * @param ddepth  {number?} 目标图片所需深度 默认 -1 自动计算
 * @param anchor_x {number?} 卷积核的锚点 x 默认 -1  表示中心
 * @param anchor_y {number?} 卷积核的锚点 y 默认 -1  表示中心
 * @param delta    {number?} 对结果的每个像素增加值  默认 0
 * @param borderType {number?} 边缘模式 选项有1~5,16 默认4
 * @returns {Mat|null} 锐化后的图片
 */
void Filter2D(const long addrRgb, const long addrResult, int kernel_type, int ddepth, int anchor_x,
              int anchor_y, double delta, int border_type);


/**
 * 颜色范围提取  一般和 HLS HSV 图片一起使用 RGB也可以 只是不好确定颜色范围
 * @param img   { Mat } 图片
 * @param lowerColor {number[]} 颜色下限数组 例如[30,127,90];
 * @param upperColor {number[]} 颜色上限数组 例如[90,200,130];
 * @returns {Mat} 掩膜图片 即黑白图片  可以做为 copyto 或  bitwise_and 掩膜获得彩色图片
 */
void
InRange(const long addrRgb, const long addrResult, const long _lowerColor, const long _upperColor);


/**
 * 骨架提取
 * @param img {Mat} 二值化图片
 * @param maxIterations {number?} 限制迭代次数，如果不进行限制，默认为-1
 * @returns {null|Mat}
 */
void Skeleton(const long addrRgb, const long addrResult, int max_iterations);


/**
 * 获取连通域状态
 * @param img {Mat} 二值化图片
 * @param connectivity {number?}  领域类型 4 或 8, 默认8,表示8连通
 * @returns {null|ConnectedComponents[]} 返回 null 或者 ConnectedComponents数组
 * ConnectedComponents对象 包含一下信息<br/>
 *  index:引索值 为connectedComponentsWithMat提供的<br/>
 *  center:中心的point<br/>
 *  rect:区域<br/>
 *  whiteArea:白点的面积(像素点为单位)<br/>
 *  rectArea:矩形围成的面积<br/>
 */
const char *ConnectedComponentsWithStats(const long addrRgb, int connectivity);


/**
 * 获取连通域图片
 * @param img {Mat} 二值化图片
 * @param indexArr {number[]} 引索值数组 可从connectedComponentsWithStats结果的index属性获得
 * @param connectivity {number?}  领域类型 4 或 8, 默认8
 * @returns {null|Mat } null或者图片
 */
void
ConnectedComponentsWithMat(const long addrRgb, const long addrResult, const long _contoursIndex,
                           int connectivity);


/**
 * 掩膜复制
 * @param img  { Mat } 目标图片
 * @param mask  {Mat } 掩膜图片必须和目标图片一样大小, 一般是二值化图片;
 *                        掩膜用于告诉函数复制过程中仅仅复制掩膜上非零的点所对应的坐标,
 *                        例如 掩膜上 {x:0,y:1} 是白色,{x:2,y:2} 是黑色,
 *                        那么复制的图片中{x:0,y:1}有颜色,{x:2,y:2} 未复制所以没有颜色
 *
 * @returns {Mat|null} 图片
 */
void CopyTo(const long addrRgb, const long addrResult, const long _mask);


/**
 * 图片 与
 * @param img1  {Mat} 图片1
 * @param img2  {Mat} 图片2 必须与 图片1 一样大小
 * @param mask  {Mat?}   掩膜 可选 必须与 图片1 一样大小
 * @returns {Mat|null} 根据图片1 的彩色图片
 */
void Bitwise_and(const long addrRgb1, const long addrRgb2, const long addrResult, const long _mask);
/**
 * 图片 或
 * @param img1 {Mat} 图片1
 * @param img2 {Mat} 图片2 必须与 图片1 一样大小  , 也可以是 图片1 图片
 * @param mask  {Mat?}   掩膜 可选 必须与 图片1 一样大小
 * @returns {Mat} 掩膜图片 即黑白图片  可以使用 bitwise_and 获取彩色图片
 */
void Bitwise_or(const long addrRgb1, const long addrRgb2, const long addrResult, const long _mask);
/**
 * 图片 非
 * @param img {Mat} 图片1
 * @param mask  {Mat?}   掩膜 可选 必须与 图片1 一样大小
 * @returns {Mat} 图片
 */
void Bitwise_not(const long addrRgb, const long addrResult, const long _mask);
/**
 * 图片 异或
 * @param img1 {Mat} 图片1
 * @param img2 {Mat} 图片2 必须与 图片1 一样大小
 * @param mask {Mat?}   掩膜 可选 必须与 图片1 一样大小
 * @returns {Mat} 图片
 */
void Bitwise_xor(const long addrRgb1, const long addrRgb2, const long addrResult, const long _mask);

/**
 *  获取轮廓重心
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {Point[]|null}   返回所有轮廓的重心坐标数组 或者 null

 */
const char *GetContoursMoments(const long addrRgb, int mode, int method, const long _contoursIndex);


/**
 * 透视变换，可用于矫正目标
 * @param img {Mat}  图片
 * @param srcPoint {Point[]|int[][]} Point数组 或 number的二维数组  原图要转换的目标的4个坐标的数组（注：逆时针顺序） 例如[{x:12,y:33},{x:42,y:63}...] 或[[12,33],[42,63]...]
 * @param dstPoint {Point[]|int[][]} Point数组 或 number的二维数组  转换后的4个坐标的数组（注：逆时针顺序，且与srcPoint四点一一对应） 例如[{x:0,y:0},{x:66,y:88}...] 或[[0,0],[66,88]...]
 * @param width  {number?}  转换后图片的宽带，默认根据dstPoint自动计算,一般不用填写
 * @param height {number?} 转换后图片的高度，默认根据dstPoint自动计算,一般不用填写
 * @returns {Mat|null} 转换后的图片 或者 null
 */
void WarpPerspective(const long addrRgb, const long addrResult, const long _srcPointArr,
                     const long _dstPointArr, int width, int height);


/**
 * 投影
 * @param srcMat {Mat} 二值化图片
 * @param direction {number} 0 表示垂直投影， 1 表示水平投影 默认0
 * @returns {Mat|null} 二值化图片 或 null
 */
void Projection(const long addrRgb, const long addrResult, int mode);

/**
 * 投影的值
 * @param srcMat {Mat} 二值化图片
 * @param direction {number} 0 表示垂直投影， 1 表示水平投影 默认0
 * @returns {number[]|null} 一维数组 表示每个数字表示图片一列或行上白点的数量 或 null
 */
void ProjectionValue(const long addrRgb, const long resultIntVec, int mode);



/**
 * 图片比较相似度, 只做颜色统计比较,无视大小和方向是否相同
 * @param img1
 * @param img2
 * @returns 返回相似度  0~1.0  1.0最好
 */
double ImageCompare(const long addrRgb1,const long addrRgb2);

/**
 * 检测圆形进度条的进度 顺时针检测
 * @param img {Mat} Mat图片
 * @param color  {string} 颜色字符串 例如:"#FFFFFF-0x101010"
 * @param x      {number} 圆心坐标 x
 * @param y      {number} 圆心坐标 y
 * @param r      {number} 圆形半径 r
 * @param startAngle {number?} 开始角度0~359,以12点钟方向为0度起点, -1表示自动计算起点,默认-1;
 * @param endAngle   {number?} 结束角度1~360,默认360;
 * @returns {number}  进度,0~359;
 */
int GetCircleBarValue(const long addrRgb  ,const char* _color, int x, int y, int r, int startAngle, int endAngle);


/**
 * 图片合并
 * @param img1 {Mat} 图片1
 * @param img2 {Mat} 图片2
 * @param direction {number}  方向 0表示横向,1表示纵向,默认0;
 * @returns {Mat}
 */
void MergeMat(const long  addrRgb1,const  long addrRgb2, const  long addrResult, int direction );


/**
 * 图片相加
 * @param img1 {Mat} 图1
 * @param img2 {Mat} 图2
 * @returns {Mat}  返回相加结果图片
 */
//图像加
void Add(const long  addrRgb1,const  long addrRgb2, const  long addrResult);

/**
 * 图片相减
 * @param img1 {Mat} 图1
 * @param img2 {Mat} 图2
 * @returns {Mat}  返回相减结果图片
 */
void Subtract(const long  addrRgb1,const  long addrRgb2, const  long addrResult);



/**
 * Mat转base64
 * @param addrRgb1 图片
 * @param format 格式 只支持jpg和png
 * @param q 质量0-100 只对jpg有效 默认100
 * @return base64
 */
const char* MatToBase64(const long  addrRgb1,const char* format,int q);
/**
 * base64转Mat
 * @param encoded
 * @param addrResult
 */

void Base64ToMat(const char* encoded,const  long addrResult);

/**
 * 颜色替换   只是不好确定颜色范围
 * @param img   { Mat } 图片
 * @param lowerColor {number[]} 颜色下限数组 例如[30,127,90];
 * @param upperColor {number[]} 颜色上限数组 例如[90,200,130];
 * @param color RGB颜色 与_mode参数无关 如“#FF0000”
 * @param sx
 * @param sy
 * @param ex
 * @param ey
 * @param _mode 内部lowerColor upperColor参数使用方式 HSV HLS RGB
 */

void ReplaceColor(const long addrRgb, const long addrResult, const long _lowerColor,
                  const long _upperColor, char *_color,int sx, int sy, int ex, int ey, double alpha,char *_mode);


/**
 * 保存图片到文件
 * @param imgAddress Mat对象地址
 * @param filename 保存路径（如 /sdcard/result.png）
 * @param quality 保存质量（1-100，png格式忽略此参数）
 * @param format jpg或png
 * @return 1=成功，0=失败
 */
int SaveMat(const long imgAddress, const char *filePath,const char *format, int quality);

#ifdef __cplusplus
}
#endif


#endif //OPENCV430_IMAGEHANDLER_H
