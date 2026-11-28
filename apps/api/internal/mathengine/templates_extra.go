package mathengine

import (
	"fmt"
	"math/rand"
)

func buildMCQ(
	code string,
	seed int64,
	grade int,
	skillCode string,
	skillName LocaleText,
	curriculumCode string,
	difficulty int,
	locale string,
	stem string,
	options []MCQOption,
	correctKey string,
	hints []string,
	steps []SolutionStep,
) GeneratedProblem {
	return GeneratedProblem{
		Question: RenderedQuestion{
			ID:             fmt.Sprintf("PL-G%d-%s-%d", grade, code, seed),
			TemplateCode:   code,
			Seed:           seed,
			Grade:          grade,
			SkillCode:      skillCode,
			SkillName:      skillName.For(locale),
			CurriculumCode: curriculumCode,
			Difficulty:     difficulty,
			InputType:      InputMCQ,
			Locale:         locale,
			Stem:           stem,
			TTSText:        VerbalizeMath(stem, locale),
			Hints:          hints,
			Options:        options,
		},
		Solution: WorkedSolution{
			CorrectAnswers: map[string]string{"choice": correctKey},
			Steps:          steps,
		},
		GradingStrategy: "exact_choice",
	}
}

func buildMultiNumeric(
	code string,
	seed int64,
	grade int,
	skillCode string,
	skillName LocaleText,
	curriculumCode string,
	difficulty int,
	locale string,
	stem string,
	parts []InputPart,
	answers map[string]string,
	hints []string,
	steps []SolutionStep,
) GeneratedProblem {
	return GeneratedProblem{
		Question: RenderedQuestion{
			ID:             fmt.Sprintf("PL-G%d-%s-%d", grade, code, seed),
			TemplateCode:   code,
			Seed:           seed,
			Grade:          grade,
			SkillCode:      skillCode,
			SkillName:      skillName.For(locale),
			CurriculumCode: curriculumCode,
			Difficulty:     difficulty,
			InputType:      InputMultiNumeric,
			Locale:         locale,
			Stem:           stem,
			TTSText:        VerbalizeMath(stem, locale),
			Hints:          hints,
			Parts:          parts,
		},
		Solution: WorkedSolution{
			CorrectAnswers: answers,
			Steps:          steps,
		},
		GradingStrategy: "exact_multi_numeric",
	}
}

func buildFraction(
	code string,
	seed int64,
	grade int,
	skillCode string,
	skillName LocaleText,
	curriculumCode string,
	difficulty int,
	locale string,
	stem string,
	num, den int,
	hints []string,
	steps []SolutionStep,
) GeneratedProblem {
	return GeneratedProblem{
		Question: RenderedQuestion{
			ID:             fmt.Sprintf("PL-G%d-%s-%d", grade, code, seed),
			TemplateCode:   code,
			Seed:           seed,
			Grade:          grade,
			SkillCode:      skillCode,
			SkillName:      skillName.For(locale),
			CurriculumCode: curriculumCode,
			Difficulty:     difficulty,
			InputType:      InputFraction,
			Locale:         locale,
			Stem:           stem,
			TTSText:        VerbalizeMath(stem, locale),
			Hints:          hints,
			Parts: []InputPart{
				{Key: "numerator", Label: LocaleText{EN: "Numerator", ID: "Pembilang"}.For(locale)},
				{Key: "denominator", Label: LocaleText{EN: "Denominator", ID: "Penyebut"}.For(locale)},
			},
		},
		Solution: WorkedSolution{
			CorrectAnswers: map[string]string{"numerator": fmt.Sprintf("%d", num), "denominator": fmt.Sprintf("%d", den)},
			Steps:          steps,
		},
		GradingStrategy: "equivalent_fraction",
	}
}

func extraTemplates() []TemplateMeta {
	return []TemplateMeta{
		// ==================== GRADE 1 (+6 -> 10 total) ====================
		{
			Code: "g1_number_bonds_10", Grade: 1, SkillCode: "num-add-within-20",
			SkillName:      LocaleText{EN: "Number Bonds to 10", ID: "Pasangan Bilangan 10"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 1, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 1 + rng.Intn(9)
				ans := 10 - a
				stem := LocaleText{
					EN: fmt.Sprintf("What number makes 10 together with %d? (%d + ___ = 10)", a, a),
					ID: fmt.Sprintf("Bilangan apa yang berpasangan dengan %d untuk menjadi 10? (%d + ___ = 10)", a, a),
				}.For(locale)
				return buildSingleNumeric("g1_number_bonds_10", seed, 1, "num-add-within-20",
					LocaleText{EN: "Number Bonds to 10", ID: "Pasangan Bilangan 10"}, "SINGAPORE_MOE", 1, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), "",
					[]string{LocaleText{EN: "Count from the number up to 10.", ID: "Hitung maju sampai 10."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("10 − %d = %d", a, ans)}})
			},
		},
		{
			Code: "g1_compare_numbers_mcq", Grade: 1, SkillCode: "num-place-value-100",
			SkillName:      LocaleText{EN: "Comparing Two-Digit Numbers", ID: "Membandingkan Bilangan Dua Angka"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 1, InputType: InputMCQ,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 12 + rng.Intn(30)
				b := a + 3 + rng.Intn(15)
				c := a - 2 - rng.Intn(8)
				d := a + 1
				stem := LocaleText{
					EN: fmt.Sprintf("Which number is the GREATEST among %d, %d, %d, and %d?", a, b, c, d),
					ID: fmt.Sprintf("Manakah bilangan PALING BESAR di antara %d, %d, %d, dan %d?", a, b, c, d),
				}.For(locale)
				opts := []MCQOption{
					{Key: "A", Label: fmt.Sprintf("%d", a)},
					{Key: "B", Label: fmt.Sprintf("%d", b)},
					{Key: "C", Label: fmt.Sprintf("%d", c)},
					{Key: "D", Label: fmt.Sprintf("%d", d)},
				}
				return buildMCQ("g1_compare_numbers_mcq", seed, 1, "num-place-value-100",
					LocaleText{EN: "Comparing Two-Digit Numbers", ID: "Membandingkan Bilangan Dua Angka"}, "CAMBRIDGE_0096", 1, locale, stem, opts, "B",
					[]string{LocaleText{EN: "Compare the tens digit first.", ID: "Bandingkan angka puluhan terlebih dahulu."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d is greater than %d, %d, and %d.", b, a, c, d)}})
			},
		},
		{
			Code: "g1_doubles_within_20", Grade: 1, SkillCode: "num-add-within-20",
			SkillName:      LocaleText{EN: "Doubles Facts", ID: "Penjumlahan Bilangan Kembar"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				n := 3 + rng.Intn(8)
				ans := n * 2
				stem := LocaleText{
					EN: fmt.Sprintf("Two twin rockets each carry %d fuel pods. How many fuel pods are there in total?", n),
					ID: fmt.Sprintf("Dua roket kembar masing-masing membawa %d kapsul bahan bakar. Berapa total kapsul bahan bakar?", n),
				}.For(locale)
				return buildSingleNumeric("g1_doubles_within_20", seed, 1, "num-add-within-20",
					LocaleText{EN: "Doubles Facts", ID: "Penjumlahan Bilangan Kembar"}, "CAMBRIDGE_0096", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "pods", ID: "kapsul"}.For(locale),
					[]string{LocaleText{EN: "Add the number to itself.", ID: "Jumlahkan bilangan itu dengan dirinya sendiri."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d + %d = %d", n, n, ans)}})
			},
		},
		{
			Code: "g1_missing_subtrahend", Grade: 1, SkillCode: "num-sub-within-20",
			SkillName:      LocaleText{EN: "Missing Part Subtraction", ID: "Pengurangan Bilangan Hilang"},
			CurriculumCode: "CHALLENGING_MATHS", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				total := 12 + rng.Intn(8)
				rem := 4 + rng.Intn(6)
				ans := total - rem
				stem := LocaleText{
					EN: fmt.Sprintf("%d − ___ = %d. What is the missing number?", total, rem),
					ID: fmt.Sprintf("%d − ___ = %d. Berapakah bilangan yang hilang?", total, rem),
				}.For(locale)
				return buildSingleNumeric("g1_missing_subtrahend", seed, 1, "num-sub-within-20",
					LocaleText{EN: "Missing Part Subtraction", ID: "Pengurangan Bilangan Hilang"}, "CHALLENGING_MATHS", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), "",
					[]string{LocaleText{EN: "Subtract the result from the starting number.", ID: "Kurangi bilangan awal dengan hasilnya."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d − %d = %d", total, rem, ans)}})
			},
		},
		{
			Code: "g1_skip_count_5", Grade: 1, SkillCode: "hots-look-for-patterns-g1",
			SkillName:      LocaleText{EN: "Skip Counting by 5s", ID: "Membilang Loncat 5"},
			CurriculumCode: "EXPLORER_SERIES", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				start := (1 + rng.Intn(6)) * 5
				ans := start + 15
				stem := LocaleText{
					EN: fmt.Sprintf("A rover hops by 5s: %d, %d, %d, ___. What number comes next?", start, start+5, start+10),
					ID: fmt.Sprintf("Sebuah rover melompat 5 demi 5: %d, %d, %d, ___. Bilangan berapa berikutnya?", start, start+5, start+10),
				}.For(locale)
				return buildSingleNumeric("g1_skip_count_5", seed, 1, "hots-look-for-patterns-g1",
					LocaleText{EN: "Skip Counting by 5s", ID: "Membilang Loncat 5"}, "EXPLORER_SERIES", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), "",
					[]string{LocaleText{EN: "Add 5 to the last number.", ID: "Tambahkan 5 ke bilangan terakhir."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d + 5 = %d", start+10, ans)}})
			},
		},
		{
			Code: "g1_ordinal_position", Grade: 1, SkillCode: "num-ordinal-position",
			SkillName:      LocaleText{EN: "Counting Positions", ID: "Urutan Posisi"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				front := 2 + rng.Intn(6)
				behind := 2 + rng.Intn(6)
				total := front + 1 + behind
				name := pickName(rng)
				stem := LocaleText{
					EN: fmt.Sprintf("In a line of astronauts, %d people stand in front of %s and %d people stand behind %s. How many astronauts are in the line?", front, name, behind, name),
					ID: fmt.Sprintf("Dalam barisan astronot, ada %d orang di depan %s dan %d orang di belakang %s. Berapa total astronot dalam barisan?", front, name, behind, name),
				}.For(locale)
				return buildSingleNumeric("g1_ordinal_position", seed, 1, "num-ordinal-position",
					LocaleText{EN: "Counting Positions", ID: "Urutan Posisi"}, "KURIKULUM_MERDEKA", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", total), LocaleText{EN: "astronauts", ID: "astronot"}.For(locale),
					[]string{LocaleText{EN: "Remember to count the astronaut in the middle too!", ID: "Jangan lupa menghitung astronot yang berada di tengah!" + ""}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d + 1 + %d = %d", front, behind, total)}})
			},
		},

		// ==================== GRADE 2 (+6 -> 10 total) ====================
		{
			Code: "g2_sub_within_1000", Grade: 2, SkillCode: "num-sub-1000",
			SkillName:      LocaleText{EN: "Subtraction within 1,000", ID: "Pengurangan sampai 1.000"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 420 + rng.Intn(400)
				b := 115 + rng.Intn(250)
				ans := a - b
				stem := LocaleText{
					EN: fmt.Sprintf("A space station stored %d oxygen tanks and used %d of them. How many tanks remain?", a, b),
					ID: fmt.Sprintf("Stasiun luar angkasa menyimpan %d tabung oksigen dan memakai %d tabung. Berapa tabung yang tersisa?", a, b),
				}.For(locale)
				return buildSingleNumeric("g2_sub_within_1000", seed, 2, "num-sub-1000",
					LocaleText{EN: "Subtraction within 1,000", ID: "Pengurangan sampai 1.000"}, "SINGAPORE_MOE", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "tanks", ID: "tabung"}.For(locale),
					[]string{LocaleText{EN: "Subtract ones, tens, then hundreds.", ID: "Kurangi satuan, puluhan, lalu ratusan."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d − %d = %d", a, b, ans)}})
			},
		},
		{
			Code: "g2_equal_sharing_div", Grade: 2, SkillCode: "num-div-equal-sharing",
			SkillName:      LocaleText{EN: "Equal Sharing Division", ID: "Pembagian Sama Rata"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				groups := 2 + rng.Intn(4)
				each := 3 + rng.Intn(8)
				total := groups * each
				stem := LocaleText{
					EN: fmt.Sprintf("%d star crystals are shared equally into %d boxes. How many crystals are in each box?", total, groups),
					ID: fmt.Sprintf("%d kristal bintang dibagi rata ke dalam %d kotak. Berapa kristal di setiap kotak?", total, groups),
				}.For(locale)
				return buildSingleNumeric("g2_equal_sharing_div", seed, 2, "num-div-equal-sharing",
					LocaleText{EN: "Equal Sharing Division", ID: "Pembagian Sama Rata"}, "CAMBRIDGE_0096", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", each), LocaleText{EN: "crystals", ID: "kristal"}.For(locale),
					[]string{LocaleText{EN: "Divide the total by the number of boxes.", ID: "Bagi total kristal dengan jumlah kotak."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d ÷ %d = %d", total, groups, each)}})
			},
		},
		{
			Code: "g2_length_meters_cm", Grade: 2, SkillCode: "measure-length-m-cm",
			SkillName:      LocaleText{EN: "Meters & Centimeters", ID: "Meter & Sentimeter"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 2, InputType: InputMultiNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				m := 2 + rng.Intn(5)
				cm := 15 + rng.Intn(75)
				totalCM := m*100 + cm
				stem := LocaleText{
					EN: fmt.Sprintf("A solar cable is %d cm long. Express its length in meters and centimeters.", totalCM),
					ID: fmt.Sprintf("Panjang kabel surya adalah %d cm. Nyatakan panjangnya dalam meter dan sentimeter.", totalCM),
				}.For(locale)
				return buildMultiNumeric("g2_length_meters_cm", seed, 2, "measure-length-m-cm",
					LocaleText{EN: "Meters & Centimeters", ID: "Meter & Sentimeter"}, "KURIKULUM_MERDEKA", 2, locale, stem,
					[]InputPart{
						{Key: "m", Label: "m"},
						{Key: "cm", Label: "cm"},
					},
					map[string]string{"m": fmt.Sprintf("%d", m), "cm": fmt.Sprintf("%d", cm)},
					[]string{LocaleText{EN: "1 meter = 100 centimeters.", ID: "1 meter = 100 sentimeter."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d cm = %d m %d cm", totalCM, m, cm)}})
			},
		},
		{
			Code: "g2_fraction_build", Grade: 2, SkillCode: "frac-write",
			SkillName:      LocaleText{EN: "Writing Fractions", ID: "Menulis Pecahan"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 2, InputType: InputFraction,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				den := 3 + rng.Intn(5)
				num := 1 + rng.Intn(den-1)
				g := gcd(num, den)
				sNum, sDen := num/g, den/g
				stem := LocaleText{
					EN: fmt.Sprintf("A cosmic pie is cut into %d equal slices. %d slices are eaten. What fraction of the pie is eaten?", den, num),
					ID: fmt.Sprintf("Sebuah pai kosmik dipotong menjadi %d bagian sama besar. %d bagian dimakan. Berapa pecahan pai yang dimakan?", den, num),
				}.For(locale)
				return buildFraction("g2_fraction_build", seed, 2, "frac-write",
					LocaleText{EN: "Writing Fractions", ID: "Menulis Pecahan"}, "CAMBRIDGE_0096", 2, locale, stem, sNum, sDen,
					[]string{LocaleText{EN: "Numerator is the eaten slices; denominator is the total slices.", ID: "Pembilang adalah bagian yang dimakan; penyebut adalah total bagian."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d/%d = %d/%d", num, den, sNum, sDen)}})
			},
		},
		{
			Code: "g2_two_step_coins", Grade: 2, SkillCode: "wp-add-sub-two-step",
			SkillName:      LocaleText{EN: "Two-Step Word Problem", ID: "Soal Cerita Dua Langkah"},
			CurriculumCode: "CHALLENGING_MATHS", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				start := 40 + rng.Intn(30)
				gain := 15 + rng.Intn(20)
				spend := 10 + rng.Intn(20)
				ans := start + gain - spend
				name := pickName(rng)
				stem := LocaleText{
					EN: fmt.Sprintf("%s had %d Astro Coins, earned %d more, then spent %d coins on a badge. How many coins does %s have left?", name, start, gain, spend, name),
					ID: fmt.Sprintf("%s memiliki %d Koin Astro, mendapat %d koin lagi, lalu membelanjakan %d koin untuk lencana. Berapa sisa koin %s?", name, start, gain, spend, name),
				}.For(locale)
				return buildSingleNumeric("g2_two_step_coins", seed, 2, "wp-add-sub-two-step",
					LocaleText{EN: "Two-Step Word Problem", ID: "Soal Cerita Dua Langkah"}, "CHALLENGING_MATHS", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "coins", ID: "koin"}.For(locale),
					[]string{LocaleText{EN: "Add first, then subtract what was spent.", ID: "Jumlahkan dulu, lalu kurangi dengan yang dibelanjakan."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d + %d − %d = %d", start, gain, spend, ans)}})
			},
		},
		{
			Code: "g2_clock_minutes", Grade: 2, SkillCode: "measure-elapsed-minutes",
			SkillName:      LocaleText{EN: "Elapsed Minutes", ID: "Selisih Waktu Menit"},
			CurriculumCode: "EXPLORER_SERIES", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				startMin := 10 + rng.Intn(15)
				dur := 15 + rng.Intn(25)
				endMin := startMin + dur
				end := fmt.Sprintf("%02d:%02d", 9+endMin/60, endMin%60)
				stem := LocaleText{
					EN: fmt.Sprintf("A rocket launch countdown starts at 09:%02d and finishes at %s. How many minutes did it take?", startMin, end),
					ID: fmt.Sprintf("Hitung mundur peluncuran roket dimulai pukul 09:%02d dan selesai pukul %s. Berapa menit waktu yang dibutuhkan?", startMin, end),
				}.For(locale)
				return buildSingleNumeric("g2_clock_minutes", seed, 2, "measure-elapsed-minutes",
					LocaleText{EN: "Elapsed Minutes", ID: "Selisih Waktu Menit"}, "EXPLORER_SERIES", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", dur), LocaleText{EN: "minutes", ID: "menit"}.For(locale),
					[]string{LocaleText{EN: "Count the minutes from the starting time to the ending time.", ID: "Hitung menit dari waktu mulai hingga waktu selesai."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%s − 09:%02d = %d %s", end, startMin, dur, LocaleText{EN: "minutes", ID: "menit"}.For(locale))}})
			},
		},

		// ==================== GRADE 3 (+6 -> 10 total) ====================
		{
			Code: "g3_add_sub_4digit", Grade: 3, SkillCode: "num-add-4digit",
			SkillName:      LocaleText{EN: "4-Digit Addition", ID: "Penjumlahan 4 Angka"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 1250 + rng.Intn(2500)
				b := 1120 + rng.Intn(2200)
				ans := a + b
				stem := LocaleText{
					EN: fmt.Sprintf("Satellite A traveled %d km and Satellite B traveled %d km. What is their combined distance in km?", a, b),
					ID: fmt.Sprintf("Satelit A menempuh %d km dan Satelit B menempuh %d km. Berapa total jarak tempuh keduanya dalam km?", a, b),
				}.For(locale)
				return buildSingleNumeric("g3_add_sub_4digit", seed, 3, "num-add-4digit",
					LocaleText{EN: "4-Digit Addition", ID: "Penjumlahan 4 Angka"}, "KURIKULUM_MERDEKA", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), "km",
					[]string{LocaleText{EN: "Add column by column from right to left.", ID: "Jumlahkan per nilai tempat dari kanan ke kiri."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d + %d = %d", a, b, ans)}})
			},
		},
		{
			Code: "g3_perimeter_square", Grade: 3, SkillCode: "geo-area-perimeter-rect",
			SkillName:      LocaleText{EN: "Perimeter of a Square", ID: "Keliling Persegi"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				side := 6 + rng.Intn(14)
				p := 4 * side
				stem := LocaleText{
					EN: fmt.Sprintf("A square solar panel has side length %d m. What is its perimeter in meters?", side),
					ID: fmt.Sprintf("Panel surya berbentuk persegi memiliki panjang sisi %d m. Berapa kelilingnya dalam meter?", side),
				}.For(locale)
				return buildSingleNumeric("g3_perimeter_square", seed, 3, "geo-area-perimeter-rect",
					LocaleText{EN: "Perimeter of a Square", ID: "Keliling Persegi"}, "CAMBRIDGE_0096", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", p), "m",
					[]string{LocaleText{EN: "A square has 4 equal sides.", ID: "Persegi memiliki 4 sisi yang sama panjang."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("4 × %d = %d m", side, p)}})
			},
		},
		{
			Code: "g3_fraction_add_input", Grade: 3, SkillCode: "frac-add-like",
			SkillName:      LocaleText{EN: "Adding Like Fractions", ID: "Menjumlahkan Pecahan Berpenyebut Sama"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 3, InputType: InputFraction,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				den := 5 + rng.Intn(5)
				a := 1 + rng.Intn(den/2)
				b := 1 + rng.Intn(den-a)
				num := a + b
				g := gcd(num, den)
				stem := LocaleText{
					EN: fmt.Sprintf("Write %d/%d + %d/%d as a fraction.", a, den, b, den),
					ID: fmt.Sprintf("Tuliskan hasil %d/%d + %d/%d dalam bentuk pecahan.", a, den, b, den),
				}.For(locale)
				return buildFraction("g3_fraction_add_input", seed, 3, "frac-add-like",
					LocaleText{EN: "Adding Like Fractions", ID: "Menjumlahkan Pecahan Berpenyebut Sama"}, "SINGAPORE_MOE", 3, locale, stem, num/g, den/g,
					[]string{LocaleText{EN: "Add the numerators and keep the denominator.", ID: "Jumlahkan pembilang dan pertahankan penyebutnya."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("(%d + %d)/%d = %d/%d", a, b, den, num/g, den/g)}})
			},
		},
		{
			Code: "g3_mass_kg_g", Grade: 3, SkillCode: "measure-mass-kg-g",
			SkillName:      LocaleText{EN: "Mass in kg & g", ID: "Massa dalam kg & g"},
			CurriculumCode: "EXPLORER_SERIES", Difficulty: 3, InputType: InputMultiNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				kg := 2 + rng.Intn(7)
				gr := 150 + rng.Intn(750)
				total := kg*1000 + gr
				stem := LocaleText{
					EN: fmt.Sprintf("A moon rock weighs %d g. Convert its mass into kilograms and grams.", total),
					ID: fmt.Sprintf("Sebuah batu bulan memiliki massa %d g. Ubah massanya menjadi kilogram dan gram.", total),
				}.For(locale)
				return buildMultiNumeric("g3_mass_kg_g", seed, 3, "measure-mass-kg-g",
					LocaleText{EN: "Mass in kg & g", ID: "Massa dalam kg & g"}, "EXPLORER_SERIES", 3, locale, stem,
					[]InputPart{{Key: "kg", Label: "kg"}, {Key: "g", Label: "g"}},
					map[string]string{"kg": fmt.Sprintf("%d", kg), "g": fmt.Sprintf("%d", gr)},
					[]string{LocaleText{EN: "1 kg = 1,000 g.", ID: "1 kg = 1.000 g."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d g = %d kg %d g", total, kg, gr)}})
			},
		},
		{
			Code: "g3_hots_sum_diff", Grade: 3, SkillCode: "wp-sum-difference",
			SkillName:      LocaleText{EN: "Sum & Difference Model", ID: "Model Jumlah & Selisih"},
			CurriculumCode: "HOTS_WHEEL", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				small := 12 + rng.Intn(20)
				diff := 6 + rng.Intn(10)
				large := small + diff
				sum := small + large
				stem := LocaleText{
					EN: fmt.Sprintf("Two crates have %d crystals in total. The larger crate has %d more crystals than the smaller crate. How many crystals are in the larger crate?", sum, diff),
					ID: fmt.Sprintf("Dua peti berisi total %d kristal. Peti besar berisi %d kristal lebih banyak dari peti kecil. Berapa kristal di peti besar?", sum, diff),
				}.For(locale)
				return buildSingleNumeric("g3_hots_sum_diff", seed, 3, "wp-sum-difference",
					LocaleText{EN: "Sum & Difference Model", ID: "Model Jumlah & Selisih"}, "HOTS_WHEEL", 4, "DRAW_MODELS", LocaleText{EN: "Draw Models", ID: "Menggambar Model Batang"},
					locale, stem, fmt.Sprintf("%d", large), LocaleText{EN: "crystals", ID: "kristal"}.For(locale),
					[]string{LocaleText{EN: "Add the difference to the sum, then divide by 2.", ID: "Tambahkan selisih ke jumlah total, lalu bagi 2."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("(%d + %d) ÷ 2 = %d", sum, diff, large)}})
			},
		},
		{
			Code: "g3_multiple_mcq", Grade: 3, SkillCode: "num-identify-multiples",
			SkillName:      LocaleText{EN: "Multiples Check", ID: "Mengenal Kelipatan"},
			CurriculumCode: "CHALLENGING_MATHS", Difficulty: 3, InputType: InputMCQ,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				base := 7 + rng.Intn(3)
				mult := base * (4 + rng.Intn(5))
				opts := []MCQOption{
					{Key: "A", Label: fmt.Sprintf("%d", mult-1)},
					{Key: "B", Label: fmt.Sprintf("%d", mult+2)},
					{Key: "C", Label: fmt.Sprintf("%d", mult)},
					{Key: "D", Label: fmt.Sprintf("%d", mult+1)},
				}
				stem := LocaleText{
					EN: fmt.Sprintf("Which of the following numbers is a multiple of %d?", base),
					ID: fmt.Sprintf("Bilangan manakah yang merupakan kelipatan %d?", base),
				}.For(locale)
				return buildMCQ("g3_multiple_mcq", seed, 3, "num-identify-multiples",
					LocaleText{EN: "Multiples Check", ID: "Mengenal Kelipatan"}, "CHALLENGING_MATHS", 3, locale, stem, opts, "C",
					[]string{LocaleText{EN: fmt.Sprintf("Divide each option by %d.", base), ID: fmt.Sprintf("Bagi setiap pilihan dengan %d.", base)}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d ÷ %d = %d (no remainder)", mult, base, mult/base)}})
			},
		},

		// ==================== GRADE 4 (+6 -> 10 total) ====================
		{
			Code: "g4_mul_2digit", Grade: 4, SkillCode: "num-mul-2digit",
			SkillName:      LocaleText{EN: "2-Digit Multiplication", ID: "Perkalian 2 Angka"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 14 + rng.Intn(25)
				b := 12 + rng.Intn(15)
				ans := a * b
				stem := LocaleText{
					EN: fmt.Sprintf("A fleet has %d shuttles and each shuttle carries %d passengers. How many passengers can fly together?", a, b),
					ID: fmt.Sprintf("Sebuah armada memiliki %d pesawat dan setiap pesawat mengangkut %d penumpang. Berapa total penumpang?", a, b),
				}.For(locale)
				return buildSingleNumeric("g4_mul_2digit", seed, 4, "num-mul-2digit",
					LocaleText{EN: "2-Digit Multiplication", ID: "Perkalian 2 Angka"}, "SINGAPORE_MOE", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "passengers", ID: "penumpang"}.For(locale),
					[]string{LocaleText{EN: "Multiply tens and ones separately, then add.", ID: "Kalikan puluhan dan satuan secara terpisah lalu jumlahkan."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d × %d = %d", a, b, ans)}})
			},
		},
		{
			Code: "g4_gcf_two_numbers", Grade: 4, SkillCode: "num-factors-multiples",
			SkillName:      LocaleText{EN: "Greatest Common Factor", ID: "Faktor Persekutuan Terbesar (FPB)"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				g := 4 + rng.Intn(6)
				a := g * (2 + rng.Intn(3))
				b := g * (3 + rng.Intn(4))
				actualG := gcd(a, b)
				stem := LocaleText{
					EN: fmt.Sprintf("Find the Greatest Common Factor (GCF) of %d and %d.", a, b),
					ID: fmt.Sprintf("Tentukan Faktor Persekutuan Terbesar (FPB) dari %d dan %d.", a, b),
				}.For(locale)
				return buildSingleNumeric("g4_gcf_two_numbers", seed, 4, "num-factors-multiples",
					LocaleText{EN: "Greatest Common Factor", ID: "Faktor Persekutuan Terbesar (FPB)"}, "KURIKULUM_MERDEKA", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", actualG), "",
					[]string{LocaleText{EN: "List common factors of both numbers.", ID: "Daftarkan faktor persekutuan dari kedua bilangan."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("GCF(%d, %d) = %d", a, b, actualG)}})
			},
		},
		{
			Code: "g4_angles_triangle_mcq", Grade: 4, SkillCode: "geo-triangle-angles",
			SkillName:      LocaleText{EN: "Angles in a Triangle", ID: "Sudut pada Segitiga"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 3, InputType: InputMCQ,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 35 + rng.Intn(35)
				b := 45 + rng.Intn(35)
				c := 180 - a - b
				d := 90
				if d == c-10 || d == c || d == c+10 {
					d = c + 20
				}
				stem := LocaleText{
					EN: fmt.Sprintf("A triangular fin has two angles measuring %d° and %d°. What is the third angle?", a, b),
					ID: fmt.Sprintf("Sirip roket berbentuk segitiga memiliki dua sudut %d° dan %d°. Berapa besar sudut ketiga?", a, b),
				}.For(locale)
				opts := []MCQOption{
					{Key: "A", Label: fmt.Sprintf("%d°", c-10)},
					{Key: "B", Label: fmt.Sprintf("%d°", c)},
					{Key: "C", Label: fmt.Sprintf("%d°", c+10)},
					{Key: "D", Label: fmt.Sprintf("%d°", d)},
				}
				return buildMCQ("g4_angles_triangle_mcq", seed, 4, "geo-triangle-angles",
					LocaleText{EN: "Angles in a Triangle", ID: "Sudut pada Segitiga"}, "CAMBRIDGE_0096", 3, locale, stem, opts, "B",
					[]string{LocaleText{EN: "The angles in any triangle add up to 180°.", ID: "Jumlah ketiga sudut segitiga adalah 180°."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("180° − %d° − %d° = %d°", a, b, c)}})
			},
		},
		{
			Code: "g4_mixed_to_improper", Grade: 4, SkillCode: "frac-mixed-to-improper",
			SkillName:      LocaleText{EN: "Mixed to Improper Fraction", ID: "Pecahan Campuran ke Biasa"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 3, InputType: InputFraction,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				whole := 2 + rng.Intn(4)
				den := 3 + rng.Intn(5)
				rem := 1 + rng.Intn(den-1)
				num := whole*den + rem
				g := gcd(num, den)
				stem := LocaleText{
					EN: fmt.Sprintf("Convert the mixed number %d %d/%d into an improper fraction.", whole, rem, den),
					ID: fmt.Sprintf("Ubahlah pecahan campuran %d %d/%d menjadi pecahan biasa.", whole, rem, den),
				}.For(locale)
				return buildFraction("g4_mixed_to_improper", seed, 4, "frac-mixed-to-improper",
					LocaleText{EN: "Mixed to Improper Fraction", ID: "Pecahan Campuran ke Biasa"}, "CAMBRIDGE_0096", 3, locale, stem, num/g, den/g,
					[]string{LocaleText{EN: "Multiply whole number by denominator, then add numerator.", ID: "Kalikan bilangan bulat dengan penyebut, lalu tambah pembilang."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("(%d × %d + %d)/%d = %d/%d", whole, den, rem, den, num/g, den/g)}})
			},
		},
		{
			Code: "g4_decimal_mul_10", Grade: 4, SkillCode: "dec-mul-whole",
			SkillName:      LocaleText{EN: "Multiplying Decimals", ID: "Perkalian Desimal"},
			CurriculumCode: "EXPLORER_SERIES", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				whole := 2 + rng.Intn(6)
				tenth := 2 + rng.Intn(7)
				qty := 4
				ansTimes10 := (whole*10 + tenth) * qty
				ansStr := fmt.Sprintf("%d.%d", ansTimes10/10, ansTimes10%10)
				stem := LocaleText{
					EN: fmt.Sprintf("One energy cell weighs %d.%d kg. What is the total mass of %d identical cells in kg?", whole, tenth, qty),
					ID: fmt.Sprintf("Satu sel energi bermassa %d,%d kg. Berapa total massa %d sel identik dalam kg?", whole, tenth, qty),
				}.For(locale)
				return buildSingleNumeric("g4_decimal_mul_10", seed, 4, "dec-mul-whole",
					LocaleText{EN: "Multiplying Decimals", ID: "Perkalian Desimal"}, "EXPLORER_SERIES", 4, "", LocaleText{},
					locale, stem, ansStr, "kg",
					[]string{LocaleText{EN: "Multiply as whole numbers, then place 1 decimal digit.", ID: "Kalikan seperti bilangan bulat lalu beri 1 angka di belakang koma."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d.%d × %d = %s kg", whole, tenth, qty, ansStr)}})
			},
		},
		{
			Code: "g4_challenging_remainder", Grade: 4, SkillCode: "num-div-quotient-remainder",
			SkillName:      LocaleText{EN: "Quotient & Remainder", ID: "Hasil Bagi & Sisa"},
			CurriculumCode: "CHALLENGING_MATHS", Difficulty: 4, InputType: InputMultiNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				div := 6 + rng.Intn(6)
				q := 11 + rng.Intn(12)
				r := 1 + rng.Intn(div-1)
				total := div*q + r
				stem := LocaleText{
					EN: fmt.Sprintf("Divide %d by %d. Enter the quotient and remainder.", total, div),
					ID: fmt.Sprintf("Bagilah %d dengan %d. Masukkan hasil bagi dan sisanya.", total, div),
				}.For(locale)
				return buildMultiNumeric("g4_challenging_remainder", seed, 4, "num-div-quotient-remainder",
					LocaleText{EN: "Quotient & Remainder", ID: "Hasil Bagi & Sisa"}, "CHALLENGING_MATHS", 4, locale, stem,
					[]InputPart{
						{Key: "q", Label: LocaleText{EN: "Quotient", ID: "Hasil bagi"}.For(locale)},
						{Key: "r", Label: LocaleText{EN: "Remainder", ID: "Sisa"}.For(locale)},
					},
					map[string]string{"q": fmt.Sprintf("%d", q), "r": fmt.Sprintf("%d", r)},
					[]string{LocaleText{EN: "Find the largest multiple of divisor <= dividend.", ID: "Cari kelipatan pembagi terbesar yang <= bilangan yang dibagi."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d = %d × %d + %d", total, div, q, r)}})
			},
		},

		// ==================== GRADE 5 (+6 -> 10 total) ====================
		{
			Code: "g5_fraction_add_unlike", Grade: 5, SkillCode: "frac-add-unlike",
			SkillName:      LocaleText{EN: "Unlike Fraction Addition", ID: "Penjumlahan Pecahan Beda Penyebut"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 3, InputType: InputFraction,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 1 + rng.Intn(2)
				b := 1 + rng.Intn(3)
				num := a*4 + b*3
				den := 12
				g := gcd(num, den)
				stem := LocaleText{
					EN: fmt.Sprintf("Write %d/3 + %d/4 as a fraction.", a, b),
					ID: fmt.Sprintf("Tuliskan hasil %d/3 + %d/4 dalam bentuk pecahan.", a, b),
				}.For(locale)
				return buildFraction("g5_fraction_add_unlike", seed, 5, "frac-add-unlike",
					LocaleText{EN: "Unlike Fraction Addition", ID: "Penjumlahan Pecahan Beda Penyebut"}, "CAMBRIDGE_0096", 3, locale, stem, num/g, den/g,
					[]string{LocaleText{EN: "Use common denominator 12.", ID: "Samakan penyebut menjadi 12."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d/12 + %d/12 = %d/%d", a*4, b*3, num/g, den/g)}})
			},
		},
		{
			Code: "g5_percentage_increase", Grade: 5, SkillCode: "pct-discount",
			SkillName:      LocaleText{EN: "Percentage Increase", ID: "Kenaikan Persentase"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				base := (4 + rng.Intn(10)) * 20
				pct := []int{10, 20, 25, 50}[rng.Intn(4)]
				ans := base + (base*pct)/100
				stem := LocaleText{
					EN: fmt.Sprintf("A moon colony had %d robots and increased its fleet by %d%%. How many robots are there now?", base, pct),
					ID: fmt.Sprintf("Koloni bulan memiliki %d robot dan menambah armadanya sebesar %d%%. Berapa jumlah robot sekarang?", base, pct),
				}.For(locale)
				return buildSingleNumeric("g5_percentage_increase", seed, 5, "pct-discount",
					LocaleText{EN: "Percentage Increase", ID: "Kenaikan Persentase"}, "SINGAPORE_MOE", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "robots", ID: "robot"}.For(locale),
					[]string{LocaleText{EN: "Compute the percentage amount, then add it to the original.", ID: "Hitung nilai persentasenya, lalu tambahkan ke jumlah awal."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d + (%d%% × %d) = %d", base, pct, base, ans)}})
			},
		},
		{
			Code: "g5_triangle_area", Grade: 5, SkillCode: "geo-triangle-area",
			SkillName:      LocaleText{EN: "Area of a Triangle", ID: "Luas Segitiga"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				base := (4 + rng.Intn(8)) * 2
				h := 5 + rng.Intn(10)
				area := (base * h) / 2
				stem := LocaleText{
					EN: fmt.Sprintf("A triangular solar sail has base %d cm and height %d cm. What is its area in cm²?", base, h),
					ID: fmt.Sprintf("Layar surya berbentuk segitiga memiliki alas %d cm dan tinggi %d cm. Berapa luasnya dalam cm²?", base, h),
				}.For(locale)
				return buildSingleNumeric("g5_triangle_area", seed, 5, "geo-triangle-area",
					LocaleText{EN: "Area of a Triangle", ID: "Luas Segitiga"}, "KURIKULUM_MERDEKA", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", area), "cm²",
					[]string{LocaleText{EN: "Area = ½ × base × height.", ID: "Luas = ½ × alas × tinggi."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("½ × %d × %d = %d cm²", base, h, area)}})
			},
		},
		{
			Code: "g5_ratio_simplify_mcq", Grade: 5, SkillCode: "ratio-two-quantities",
			SkillName:      LocaleText{EN: "Simplifying Ratios", ID: "Menyederhanakan Rasio"},
			CurriculumCode: "HOTS_WHEEL", Difficulty: 3, InputType: InputMCQ,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				k := 3 + rng.Intn(5)
				a, b := 2, 3
				if rng.Intn(2) == 1 {
					a, b = 3, 5
				}
				opts := []MCQOption{
					{Key: "A", Label: fmt.Sprintf("%d : %d", a, b)},
					{Key: "B", Label: fmt.Sprintf("%d : %d", b, a)},
					{Key: "C", Label: fmt.Sprintf("%d : %d", a+1, b+1)},
					{Key: "D", Label: fmt.Sprintf("%d : %d", a*2, b)},
				}
				stem := LocaleText{
					EN: fmt.Sprintf("Express the ratio %d : %d in simplest form.", a*k, b*k),
					ID: fmt.Sprintf("Sederhanakan perbandingan %d : %d ke bentuk paling sederhana.", a*k, b*k),
				}.For(locale)
				return buildMCQ("g5_ratio_simplify_mcq", seed, 5, "ratio-two-quantities",
					LocaleText{EN: "Simplifying Ratios", ID: "Menyederhanakan Rasio"}, "HOTS_WHEEL", 3, locale, stem, opts, "A",
					[]string{LocaleText{EN: "Divide both terms by their GCF.", ID: "Bagi kedua angka dengan FPB-nya."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d : %d ÷ %d = %d : %d", a*k, b*k, k, a, b)}})
			},
		},
		{
			Code: "g5_average_total", Grade: 5, SkillCode: "data-average",
			SkillName:      LocaleText{EN: "Total from Average", ID: "Mencari Total dari Rata-rata"},
			CurriculumCode: "CHALLENGING_MATHS", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				n := 4 + rng.Intn(4)
				avg := 15 + rng.Intn(20)
				total := n * avg
				stem := LocaleText{
					EN: fmt.Sprintf("The average score of %d astronauts in a simulation is %d points. What is their total score?", n, avg),
					ID: fmt.Sprintf("Nilai rata-rata %d astronot dalam simulasi adalah %d poin. Berapa total nilai mereka?", n, avg),
				}.For(locale)
				return buildSingleNumeric("g5_average_total", seed, 5, "data-average",
					LocaleText{EN: "Total from Average", ID: "Mencari Total dari Rata-rata"}, "CHALLENGING_MATHS", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", total), LocaleText{EN: "points", ID: "poin"}.For(locale),
					[]string{LocaleText{EN: "Total = Average × Count.", ID: "Total = Rata-rata × Banyak data."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d × %d = %d", n, avg, total)}})
			},
		},
		{
			Code: "g5_unit_conversion_rate", Grade: 5, SkillCode: "rate-volume-time",
			SkillName:      LocaleText{EN: "Water Tank Rate", ID: "Debit Pengisian Tangki"},
			CurriculumCode: "EXPLORER_SERIES", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				rate := 12 + rng.Intn(15)
				mins := 5 + rng.Intn(8)
				total := rate * mins
				stem := LocaleText{
					EN: fmt.Sprintf("Water flows into a habitat tank at %d liters per minute. How many liters enter in %d minutes?", rate, mins),
					ID: fmt.Sprintf("Air mengalir ke tangki habitat dengan debit %d liter per menit. Berapa liter air yang masuk dalam %d menit?", rate, mins),
				}.For(locale)
				return buildSingleNumeric("g5_unit_conversion_rate", seed, 5, "rate-volume-time",
					LocaleText{EN: "Water Tank Rate", ID: "Debit Pengisian Tangki"}, "EXPLORER_SERIES", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", total), LocaleText{EN: "liters", ID: "liter"}.For(locale),
					[]string{LocaleText{EN: "Multiply rate by time.", ID: "Kalikan debit dengan waktu."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d × %d = %d liters", rate, mins, total)}})
			},
		},

		// ==================== GRADE 6 (+6 -> 10 total) ====================
		{
			Code: "g6_algebra_two_step", Grade: 6, SkillCode: "alg-linear-eq",
			SkillName:      LocaleText{EN: "Two-Step Algebra", ID: "Aljabar Dua Langkah"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				x := 4 + rng.Intn(12)
				m := 2 + rng.Intn(4)
				c := 3 + rng.Intn(9)
				rhs := m*x + c
				stem := LocaleText{
					EN: fmt.Sprintf("Solve for x: %dx + %d = %d", m, c, rhs),
					ID: fmt.Sprintf("Tentukan nilai x: %dx + %d = %d", m, c, rhs),
				}.For(locale)
				return buildSingleNumeric("g6_algebra_two_step", seed, 6, "alg-linear-eq",
					LocaleText{EN: "Two-Step Algebra", ID: "Aljabar Dua Langkah"}, "CAMBRIDGE_0096", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", x), "",
					[]string{LocaleText{EN: fmt.Sprintf("Subtract %d from both sides first.", c), ID: fmt.Sprintf("Kurangi kedua ruas dengan %d terlebih dahulu.", c)}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("%dx = %d → x = %d", m, rhs-c, x)}})
			},
		},
		{
			Code: "g6_fraction_divide", Grade: 6, SkillCode: "frac-divide",
			SkillName:      LocaleText{EN: "Dividing Fractions", ID: "Pembagian Pecahan"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 4, InputType: InputFraction,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 2 + rng.Intn(3)
				b := 5
				c := 1 + rng.Intn(2)
				d := 3
				num := a * d
				den := b * c
				g := gcd(num, den)
				stem := LocaleText{
					EN: fmt.Sprintf("Write (%d/%d) ÷ (%d/%d) as a fraction.", a, b, c, d),
					ID: fmt.Sprintf("Tuliskan hasil (%d/%d) ÷ (%d/%d) dalam bentuk pecahan.", a, b, c, d),
				}.For(locale)
				return buildFraction("g6_fraction_divide", seed, 6, "frac-divide",
					LocaleText{EN: "Dividing Fractions", ID: "Pembagian Pecahan"}, "SINGAPORE_MOE", 4, locale, stem, num/g, den/g,
					[]string{LocaleText{EN: "Multiply by the reciprocal of the second fraction.", ID: "Kalikan dengan kebalikan pecahan kedua."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("(%d/%d) × (%d/%d) = %d/%d", a, b, d, c, num/g, den/g)}})
			},
		},
		{
			Code: "g6_circle_area", Grade: 6, SkillCode: "geo-circle-area",
			SkillName:      LocaleText{EN: "Area of a Circle", ID: "Luas Lingkaran"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				r := 7 * (1 + rng.Intn(3))
				area := 22 * r * r / 7
				stem := LocaleText{
					EN: fmt.Sprintf("A circular radar dish has radius %d cm. Using π = 22/7, find its area in cm².", r),
					ID: fmt.Sprintf("Antena radar berbentuk lingkaran memiliki jari-jari %d cm. Dengan π = 22/7, hitung luasnya dalam cm².", r),
				}.For(locale)
				return buildSingleNumeric("g6_circle_area", seed, 6, "geo-circle-area",
					LocaleText{EN: "Area of a Circle", ID: "Luas Lingkaran"}, "KURIKULUM_MERDEKA", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", area), "cm²",
					[]string{LocaleText{EN: "Area = π × r × r.", ID: "Luas = π × r × r."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("(22/7) × %d × %d = %d cm²", r, r, area)}})
			},
		},
		{
			Code: "g6_speed_time_mcq", Grade: 6, SkillCode: "rate-speed-dst",
			SkillName:      LocaleText{EN: "Travel Time Calculation", ID: "Menghitung Waktu Tempuh"},
			CurriculumCode: "EXPLORER_SERIES", Difficulty: 4, InputType: InputMCQ,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				speed := 60 + rng.Intn(3)*15
				hrs := 2 + rng.Intn(4)
				dist := speed * hrs
				stem := LocaleText{
					EN: fmt.Sprintf("A cargo pod travels %d km at a steady speed of %d km/h. How long does the trip take?", dist, speed),
					ID: fmt.Sprintf("Kapsul kargo menempuh jarak %d km dengan kecepatan tetap %d km/jam. Berapa lama waktu tempuhnya?", dist, speed),
				}.For(locale)
				opts := []MCQOption{
					{Key: "A", Label: fmt.Sprintf("%d h", hrs-1)},
					{Key: "B", Label: fmt.Sprintf("%d h", hrs)},
					{Key: "C", Label: fmt.Sprintf("%d h", hrs+1)},
					{Key: "D", Label: fmt.Sprintf("%d h", hrs+2)},
				}
				return buildMCQ("g6_speed_time_mcq", seed, 6, "rate-speed-dst",
					LocaleText{EN: "Travel Time Calculation", ID: "Menghitung Waktu Tempuh"}, "EXPLORER_SERIES", 4, locale, stem, opts, "B",
					[]string{LocaleText{EN: "Time = Distance ÷ Speed.", ID: "Waktu = Jarak ÷ Kecepatan."}.For(locale)},
					[]SolutionStep{{Text: fmt.Sprintf("%d ÷ %d = %d hours", dist, speed, hrs)}})
			},
		},
		{
			Code: "g6_percent_discount_orig", Grade: 6, SkillCode: "pct-original-price",
			SkillName:      LocaleText{EN: "Finding Original Price", ID: "Mencari Harga Awal"},
			CurriculumCode: "HOTS_WHEEL", Difficulty: 5, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				orig := (4 + rng.Intn(10)) * 50
				sale := (orig * 80) / 100
				stem := LocaleText{
					EN: fmt.Sprintf("After a 20%% discount, a star telescope costs %d Astro Coins. What was its original price?", sale),
					ID: fmt.Sprintf("Setelah diskon 20%%, sebuah teleskop bintang berharga %d Koin Astro. Berapa harga awalnya?", sale),
				}.For(locale)
				return buildSingleNumeric("g6_percent_discount_orig", seed, 6, "pct-original-price",
					LocaleText{EN: "Finding Original Price", ID: "Mencari Harga Awal"}, "HOTS_WHEEL", 5, "WORK_BACKWARDS", LocaleText{EN: "Work Backwards", ID: "Bekerja Mundur"},
					locale, stem, fmt.Sprintf("%d", orig), LocaleText{EN: "coins", ID: "koin"}.For(locale),
					[]string{LocaleText{EN: "The sale price is 80% of the original price.", ID: "Harga diskon adalah 80% dari harga awal."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("80%% = %d → 100%% = %d ÷ 0.8 = %d", sale, sale, orig)}})
			},
		},
		{
			Code: "g6_ratio_age_diff", Grade: 6, SkillCode: "wp-constant-age-diff",
			SkillName:      LocaleText{EN: "Constant Age Difference", ID: "Selisih Umur Tetap"},
			CurriculumCode: "CHALLENGING_MATHS", Difficulty: 5, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				unit := 3 + rng.Intn(5)
				child := 2 * unit
				parent := 5 * unit
				diff := parent - child
				stem := LocaleText{
					EN: fmt.Sprintf("Commander Nova is %d years old and Pilot Raka is %d years old. In how many years will Commander Nova's age be twice Pilot Raka's age?", parent, child),
					ID: fmt.Sprintf("Komandan Nova berumur %d tahun dan Pilot Raka berumur %d tahun. Berapa tahun lagi umur Komandan Nova menjadi dua kali umur Pilot Raka?", parent, child),
				}.For(locale)
				ans := diff - child
				return buildSingleNumeric("g6_ratio_age_diff", seed, 6, "wp-constant-age-diff",
					LocaleText{EN: "Constant Age Difference", ID: "Selisih Umur Tetap"}, "CHALLENGING_MATHS", 5, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "years", ID: "tahun"}.For(locale),
					[]string{LocaleText{EN: "The age difference never changes! When Nova is 2× Raka, Raka's age equals the difference.", ID: "Selisih umur tidak pernah berubah! Saat umur Nova 2× Raka, umur Raka sama dengan selisihnya."}.For(locale)}, nil,
					[]SolutionStep{{Text: fmt.Sprintf("Diff = %d − %d = %d → %d − %d = %d years", parent, child, diff, diff, child, ans)}})
			},
		},
	}
}
